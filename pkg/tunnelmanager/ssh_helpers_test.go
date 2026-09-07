package tunnelmanager

import (
	"crypto/rand"
	"crypto/rsa"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestKnownHostsHostKeyCallbackVerifiesKnownHosts(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	knownHostsDir := filepath.Join(homeDir, ".ssh")
	if err := os.Mkdir(knownHostsDir, 0700); err != nil {
		t.Fatal(err)
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	knownHosts := "[example.test]:22 " + string(ssh.MarshalAuthorizedKey(publicKey))
	if err := os.WriteFile(filepath.Join(knownHostsDir, "known_hosts"), []byte(knownHosts), 0600); err != nil {
		t.Fatal(err)
	}

	callback, err := knownHostsHostKeyCallback()
	if err != nil {
		t.Fatal(err)
	}
	if err := callback("example.test:22", &net.TCPAddr{IP: net.ParseIP("192.0.2.1"), Port: 22}, publicKey); err != nil {
		t.Fatalf("known host key was rejected: %v", err)
	}
	if err := callback("unknown.test:22", &net.TCPAddr{IP: net.ParseIP("192.0.2.2"), Port: 22}, publicKey); err == nil {
		t.Fatal("unknown host key was accepted")
	}
}

func TestKnownHostsHostKeyCallbackReportsMissingFile(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	_, err := knownHostsHostKeyCallback()
	if err == nil {
		t.Fatal("expected missing known_hosts error")
	}
	if !strings.Contains(err.Error(), "couldn't load SSH known_hosts file") {
		t.Fatalf("unexpected error: %v", err)
	}
}
