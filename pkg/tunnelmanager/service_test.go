package tunnelmanager

import "testing"

func TestOneOffTunnelName(t *testing.T) {
	tests := []struct {
		name, machine, host string
		remote, local       int
		want                string
	}{
		{"localhost same ports", "server", "localhost", 1234, 1234, "server: 1234"},
		{"localhost differing ports", "server", "localhost", 1234, 5678, "server: 1234->5678"},
		{"remote host same ports", "server", "db.internal", 1234, 1234, "server: db.internal: 1234"},
		{"remote host differing ports", "server", "db.internal", 1234, 5678, "server: db.internal: 1234->5678"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := oneOffTunnelName(tt.machine, tt.host, tt.remote, tt.local); got != tt.want {
				t.Fatalf("oneOffTunnelName() = %q, want %q", got, tt.want)
			}
		})
	}
}
