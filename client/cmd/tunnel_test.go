package cmd

import "testing"

func TestParseDestination(t *testing.T) {
	tests := []struct {
		name        string
		destination string
		remoteHost  string
		wantHost    string
		wantPort    int
		wantErr     bool
	}{
		{name: "port defaults host", destination: "5432", wantPort: 5432},
		{name: "explicit host", destination: "db.internal:5432", wantHost: "db.internal", wantPort: 5432},
		{name: "host flag", destination: "5432", remoteHost: "db.internal", wantHost: "db.internal", wantPort: 5432},
		{name: "duplicate host", destination: "db.internal:5432", remoteHost: "other", wantErr: true},
		{name: "missing host", destination: ":5432", wantErr: true},
		{name: "invalid port", destination: "0", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port, err := parseDestination(tt.destination, tt.remoteHost)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseDestination() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && (host != tt.wantHost || port != tt.wantPort) {
				t.Fatalf("parseDestination() = (%q, %d), want (%q, %d)", host, port, tt.wantHost, tt.wantPort)
			}
		})
	}
}

func TestValidatePort(t *testing.T) {
	if err := validatePort("local port", 0, true); err != nil {
		t.Fatalf("validatePort() unexpected error: %v", err)
	}
	if err := validatePort("remote port", 0, false); err == nil {
		t.Fatal("validatePort() accepted remote port 0")
	}
	if err := validatePort("local port", 65536, true); err == nil {
		t.Fatal("validatePort() accepted port above 65535")
	}
}
