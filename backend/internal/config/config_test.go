package config

import (
	"testing"
)

func TestHTTPServerGetHost(t *testing.T) {
	tests := []struct {
		name     string
		address  string
		expected string
	}{
		{
			name:     "localhost with port",
			address:  "localhost:8080",
			expected: "localhost",
		},
		{
			name:     "0.0.0.0 with port",
			address:  "0.0.0.0:8080",
			expected: "0.0.0.0",
		},
		{
			name:     "IP address with port",
			address:  "192.168.1.1:3000",
			expected: "192.168.1.1",
		},
		{
			name:     "no port",
			address:  "localhost",
			expected: "localhost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := HTTPServer{Address: tt.address}
			result := server.GetHost()
			if result != tt.expected {
				t.Errorf("GetHost() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestHTTPServerGetPort(t *testing.T) {
	tests := []struct {
		name     string
		address  string
		expected string
	}{
		{
			name:     "localhost:8080",
			address:  "localhost:8080",
			expected: "8080",
		},
		{
			name:     "0.0.0.0:8080",
			address:  "0.0.0.0:8080",
			expected: "8080",
		},
		{
			name:     "custom port",
			address:  "localhost:3000",
			expected: "3000",
		},
		{
			name:     "no port",
			address:  "localhost",
			expected: "8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := HTTPServer{Address: tt.address}
			result := server.GetPort()
			if result != tt.expected {
				t.Errorf("GetPort() = %v, want %v", result, tt.expected)
			}
		})
	}
}
