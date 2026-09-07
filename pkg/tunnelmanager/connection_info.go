package tunnelmanager

import (
	"fmt"
	"net"
	"sync"

	"github.com/besrabasant/ssh-tunnel-manager/pkg/configmanager"
	"golang.org/x/crypto/ssh"
)

// ConnectionInfo contains the resources owned by one tunnel.
type ConnectionInfo struct {
	Client        *ssh.Client
	Listener      net.Listener
	LocalAddr     string
	RemoteAddr    string
	Connections   []net.Conn
	Config        configmanager.Entry
	Cancel        func()
	clientMu      sync.RWMutex
	listenerMu    sync.RWMutex
	connectionsMu sync.Mutex
	reconnectMu   sync.Mutex
}

func (c *ConnectionInfo) getClient() *ssh.Client {
	c.clientMu.RLock()
	defer c.clientMu.RUnlock()
	return c.Client
}

func (c *ConnectionInfo) replaceClient(client *ssh.Client) {
	c.clientMu.Lock()
	old := c.Client
	c.Client = client
	c.clientMu.Unlock()
	if old != nil && old != client {
		_ = old.Close()
	}
}

func (c *ConnectionInfo) setListener(listener net.Listener) {
	c.listenerMu.Lock()
	c.Listener = listener
	c.listenerMu.Unlock()
}

func (c *ConnectionInfo) getListener() net.Listener {
	c.listenerMu.RLock()
	defer c.listenerMu.RUnlock()
	return c.Listener
}

func (c *ConnectionInfo) AddConnection(conn net.Conn) {
	c.connectionsMu.Lock()
	c.Connections = append(c.Connections, conn)
	c.connectionsMu.Unlock()
}

func (c *ConnectionInfo) RemoveConnection(conn net.Conn) {
	c.connectionsMu.Lock()
	defer c.connectionsMu.Unlock()
	for i, active := range c.Connections {
		if active == conn {
			c.Connections = append(c.Connections[:i], c.Connections[i+1:]...)
			return
		}
	}
}

func (c *ConnectionInfo) StopClient() {
	c.clientMu.Lock()
	client := c.Client
	c.Client = nil
	c.clientMu.Unlock()
	if client != nil {
		if err := client.Close(); err != nil {
			fmt.Printf("Error closing SSH client: %v\n", err)
		}
	}
}

func (c *ConnectionInfo) StopListeners() {
	c.listenerMu.Lock()
	listener := c.Listener
	c.Listener = nil
	c.listenerMu.Unlock()
	if listener != nil {
		if err := listener.Close(); err != nil {
			fmt.Println("Error closing listener:", err)
		}
	}
}

func (c *ConnectionInfo) KillAllConnections() {
	c.connectionsMu.Lock()
	connections := c.Connections
	c.Connections = nil
	c.connectionsMu.Unlock()
	for _, conn := range connections {
		if err := conn.Close(); err != nil {
			fmt.Println("Failed to close connection:", err)
		}
	}
}

func (c *ConnectionInfo) ClearConnection() {
	if c.Cancel != nil {
		c.Cancel()
	}
	c.StopClient()
	c.StopListeners()
	c.KillAllConnections()
}
