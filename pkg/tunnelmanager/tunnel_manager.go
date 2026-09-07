package tunnelmanager

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/besrabasant/ssh-tunnel-manager/config"
	"github.com/besrabasant/ssh-tunnel-manager/pkg/configmanager"
	"golang.org/x/crypto/ssh"
)

func ensureServerAddress(addr string) string {
	if _, _, err := net.SplitHostPort(addr); err != nil && !strings.Contains(addr, ":") {
		return addr + ":" + config.DefaultSSHPort
	}
	return addr
}

func (m *tunnelManager) recreateSSHClient(ci *ConnectionInfo, port int) error {
	ci.reconnectMu.Lock()
	defer ci.reconnectMu.Unlock()
	if err := m.context.Err(); err != nil {
		return err
	}
	privateKey, err := readPrivateKeyFile(ci.Config.KeyFile)
	if err != nil {
		return err
	}
	key, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("couldn't parse private key %q: %w", ci.Config.KeyFile, err)
	}
	hostKeyCallback, err := knownHostsHostKeyCallback()
	if err != nil {
		return err
	}
	client, err := ssh.Dial("tcp", ensureServerAddress(ci.Config.Server), &ssh.ClientConfig{
		User: ci.Config.User, Auth: []ssh.AuthMethod{ssh.PublicKeys(key)},
		HostKeyCallback: hostKeyCallback, Timeout: 10 * time.Second,
	})
	if err != nil {
		return err
	}
	if m.context.Err() != nil {
		_ = client.Close()
		return m.context.Err()
	}
	ci.replaceClient(client)
	return nil
}

func (m *tunnelManager) monitorTunnel(tunnelCtx context.Context, ci *ConnectionInfo, port int) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			client := ci.getClient()
			if client == nil || probeSSHClient(client) == nil {
				continue
			}
			if err := m.recreateSSHClient(ci, port); err != nil {
				log.Printf("failed to reconnect SSH tunnel on local port %d: %v", port, err)
			}
		case <-tunnelCtx.Done():
			return
		case <-m.context.Done():
			return
		}
	}
}

func probeSSHClient(client *ssh.Client) error {
	result := make(chan error, 1)
	go func() { _, _, err := client.SendRequest("keepalive@openssh.com", true, nil); result <- err }()
	select {
	case err := <-result:
		return err
	case <-time.After(5 * time.Second):
		return context.DeadlineExceeded
	}
}

// tunnelManager serializes only map access. Network I/O is deliberately done
// after a tunnel has been reserved in the map.
type tunnelManager struct {
	Mutex        sync.RWMutex
	Connections  SSHConnections
	context      context.Context
	cancel       context.CancelFunc
	shutdownOnce sync.Once
}

func NewTunnelManager() TunnelManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &tunnelManager{Connections: make(SSHConnections), context: ctx, cancel: cancel}
}

func (m *tunnelManager) ConnectionsSnapshot() map[int]ConnectionSnapshot {
	m.Mutex.RLock()
	defer m.Mutex.RUnlock()
	result := make(map[int]ConnectionSnapshot, len(m.Connections))
	for port, ci := range m.Connections {
		result[port] = ConnectionSnapshot{LocalAddr: ci.LocalAddr, RemoteAddr: ci.RemoteAddr, Config: ci.Config}
	}
	return result
}
func (m *tunnelManager) GetConnection(port int) (ConnectionSnapshot, bool) {
	m.Mutex.RLock()
	defer m.Mutex.RUnlock()
	ci, ok := m.Connections[port]
	if !ok {
		return ConnectionSnapshot{}, false
	}
	return ConnectionSnapshot{LocalAddr: ci.LocalAddr, RemoteAddr: ci.RemoteAddr, Config: ci.Config}, true
}

// RegisterConnection atomically reserves a local port for a tunnel.
func (m *tunnelManager) RegisterConnection(port int, connection *ConnectionInfo) bool {
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	if _, exists := m.Connections[port]; exists {
		return false
	}
	m.Connections[port] = connection
	return true
}

func (m *tunnelManager) Shutdown() {
	m.shutdownOnce.Do(func() {
		m.cancel()
		m.Mutex.Lock()
		connections := m.Connections
		m.Connections = make(SSHConnections)
		m.Mutex.Unlock()
		for _, ci := range connections {
			ci.ClearConnection()
		}
	})
}

func (m *tunnelManager) StopTunneling(port int) bool {
	m.Mutex.Lock()
	ci, ok := m.Connections[port]
	if ok {
		delete(m.Connections, port)
	}
	m.Mutex.Unlock()
	if ok {
		ci.ClearConnection()
	}
	return ok
}

func (m *tunnelManager) sendResult(ch chan<- string, value string) {
	select {
	case ch <- value:
	case <-m.context.Done():
	}
}
func (m *tunnelManager) sendError(ch chan<- error, err error) {
	select {
	case ch <- err:
	case <-m.context.Done():
	}
}

func (m *tunnelManager) StartTunneling(ctx context.Context, entry configmanager.Entry, localPort int, resultChan chan<- string, errChan chan<- error) {
	defer close(resultChan)
	defer close(errChan)
	if err := ctx.Err(); err != nil {
		m.sendError(errChan, err)
		return
	}
	if err := m.context.Err(); err != nil {
		m.sendError(errChan, err)
		return
	}

	sshServer := ensureServerAddress(entry.Server)
	if _, _, err := net.SplitHostPort(sshServer); err != nil {
		m.sendError(errChan, fmt.Errorf("bad ssh server address: %w", err))
		return
	}
	remoteAddress, localAddress := fmt.Sprintf("%s:%d", entry.RemoteHost, entry.RemotePort), fmt.Sprintf("127.0.0.1:%d", localPort)
	tunnelCtx, cancel := context.WithCancel(m.context)
	ci := &ConnectionInfo{LocalAddr: localAddress, RemoteAddr: remoteAddress, Config: entry, Cancel: cancel}

	if !m.RegisterConnection(localPort, ci) {
		cancel()
		m.sendError(errChan, fmt.Errorf("connection is already open on port %d", localPort))
		return
	}
	setupSuccess := false
	defer func() {
		if !setupSuccess {
			ci.ClearConnection()
			m.Mutex.Lock()
			if m.Connections[localPort] == ci {
				delete(m.Connections, localPort)
			}
			m.Mutex.Unlock()
		}
	}()

	m.sendResult(resultChan, "Starting tunnel setup...\n")
	privateKey, err := readPrivateKeyFile(entry.KeyFile)
	if err != nil {
		m.sendError(errChan, err)
		return
	}
	key, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		m.sendError(errChan, fmt.Errorf("couldn't parse private key %q: %w", entry.KeyFile, err))
		return
	}
	hostKeyCallback, err := knownHostsHostKeyCallback()
	if err != nil {
		m.sendError(errChan, err)
		return
	}
	timeout := 10 * time.Second
	m.sendResult(resultChan, fmt.Sprintf("Connecting to %q with a timeout of %s", sshServer, timeout))
	client, err := ssh.Dial("tcp", sshServer, &ssh.ClientConfig{User: entry.User, Auth: []ssh.AuthMethod{ssh.PublicKeys(key)}, HostKeyCallback: hostKeyCallback, Timeout: timeout})
	if err != nil {
		m.sendError(errChan, fmt.Errorf("couldn't connect to SSH server %q: %w", sshServer, err))
		return
	}
	if err := ctx.Err(); err != nil {
		_ = client.Close()
		m.sendError(errChan, err)
		return
	}
	if err := tunnelCtx.Err(); err != nil {
		_ = client.Close()
		m.sendError(errChan, err)
		return
	}
	if m.context.Err() != nil {
		_ = client.Close()
		m.sendError(errChan, m.context.Err())
		return
	}
	ci.replaceClient(client)
	m.sendResult(resultChan, "\nConnected\n")

	var listener net.Listener
	for attempts := 0; attempts < 5; attempts++ {
		listener, err = net.Listen("tcp", localAddress)
		if err == nil {
			break
		}
		select {
		case <-ctx.Done():
			m.sendError(errChan, ctx.Err())
			return
		case <-m.context.Done():
			m.sendError(errChan, m.context.Err())
			return
		case <-tunnelCtx.Done():
			m.sendError(errChan, tunnelCtx.Err())
			return
		case <-time.After(time.Second):
		}
	}
	if err != nil {
		m.sendError(errChan, fmt.Errorf("couldn't set up local listener after retries: %w", err))
		return
	}
	ci.setListener(listener)
	m.Mutex.RLock()
	current := m.Connections[localPort] == ci
	m.Mutex.RUnlock()
	if !current {
		ci.ClearConnection()
		return
	}
	setupSuccess = true
	m.sendResult(resultChan, "Local listener set up, ready to accept connections.\n")
	m.sendResult(resultChan, fmt.Sprintf("Tunneling %q <==> %q through %q\n", localAddress, remoteAddress, sshServer))
	go m.forwardTunnel(tunnelCtx, ci, remoteAddress, localPort)
	go m.monitorTunnel(tunnelCtx, ci, localPort)
}

func (m *tunnelManager) forwardTunnel(tunnelCtx context.Context, ci *ConnectionInfo, remoteAddress string, localPort int) {
	defer func() {
		m.Mutex.Lock()
		if m.Connections[localPort] == ci {
			delete(m.Connections, localPort)
		}
		m.Mutex.Unlock()
	}()
	listener := ci.getListener()
	if listener == nil {
		return
	}
	for {
		localConn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) || tunnelCtx.Err() != nil || m.context.Err() != nil {
				return
			}
			log.Printf("Failed to accept local connection: %v", err)
			continue
		}
		ci.AddConnection(localConn)
		go m.forwardConnection(ci, localConn, remoteAddress, localPort)
	}
}

func (m *tunnelManager) forwardConnection(ci *ConnectionInfo, localConn net.Conn, remoteAddress string, localPort int) {
	defer ci.RemoveConnection(localConn)
	client := ci.getClient()
	if client == nil {
		_ = localConn.Close()
		return
	}
	remoteConn, err := client.Dial("tcp", remoteAddress)
	if err != nil {
		if recErr := m.recreateSSHClient(ci, localPort); recErr != nil {
			_ = localConn.Close()
			return
		}
		client = ci.getClient()
		if client == nil {
			_ = localConn.Close()
			return
		}
		remoteConn, err = client.Dial("tcp", remoteAddress)
		if err != nil {
			_ = localConn.Close()
			return
		}
	}
	runTunnel(localConn, remoteConn)
}
