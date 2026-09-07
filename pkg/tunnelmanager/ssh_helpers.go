package tunnelmanager

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// knownHostsHostKeyCallback verifies SSH server keys against the user's
// standard known_hosts file. Unknown and changed host keys are rejected.
func knownHostsHostKeyCallback() (ssh.HostKeyCallback, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("couldn't determine home directory for SSH known_hosts: %w", err)
	}

	knownHostsPath := filepath.Join(homeDir, ".ssh", "known_hosts")
	callback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("couldn't load SSH known_hosts file %q: %w (connect once with ssh or add the host with ssh-keyscan)", knownHostsPath, err)
	}
	return callback, nil
}

// Helper function to read the private key file
func readPrivateKeyFile(file string) ([]byte, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("couldn't read private key file %q: %w", file, err)
	}
	return data, nil
}

// runTunnel runs a tunnel between two connections; as soon as one connection
// reaches EOF or reports an error, both connections are closed and this
// function returns.
func runTunnel(local, remote net.Conn) {
	// Clean up
	defer local.Close()
	defer remote.Close()

	done := make(chan struct{}, 2)
	go func() {
		io.Copy(local, remote)
		done <- struct{}{}
	}()

	go func() {
		io.Copy(remote, local)
		done <- struct{}{}
	}()

	<-done
	log.Printf("Connection closed: %s", local.RemoteAddr())
}
