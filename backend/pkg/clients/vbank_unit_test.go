package clients

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

// Unit tests for VBank client (no real API calls)
// These tests will run in CI without credentials

func TestNewClient(t *testing.T) {
	logger := zerolog.Nop()

	tests := []struct {
		name         string
		baseURL      string
		clientID     string
		clientSecret string
		wantErr      bool
	}{
		{
			name:         "valid credentials",
			baseURL:      "https://api.example.com",
			clientID:     "test-client",
			clientSecret: "test-secret",
			wantErr:      false,
		},
		{
			name:         "empty base URL",
			baseURL:      "",
			clientID:     "test-client",
			clientSecret: "test-secret",
			wantErr:      false, // Client creation doesn't validate URL
		},
		{
			name:         "empty credentials",
			baseURL:      "https://api.example.com",
			clientID:     "",
			clientSecret: "",
			wantErr:      false, // Client creation doesn't validate credentials
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.baseURL, tt.clientID, tt.clientSecret, logger)

			assert.NotNil(t, client)
			assert.Equal(t, tt.baseURL, client.baseURL)
			assert.Equal(t, tt.clientID, client.clientID)
			assert.Equal(t, tt.clientSecret, client.clientSecret)
		})
	}
}

func TestClient_ContextCancellation(t *testing.T) {
	logger := zerolog.Nop()
	client := NewClient("https://api.example.com", "test", "secret", logger)

	// Test that context cancellation is respected
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// These should respect context cancellation (will fail fast)
	_, _, err := client.GetBankToken(ctx)
	assert.Error(t, err, "GetBankToken should fail with cancelled context")

	_, err = client.CreateConsent(ctx, "fake-token", "fake-redirect")
	assert.Error(t, err, "CreateConsent should fail with cancelled context")

	_, err = client.GetAccounts(ctx, "fake-token", "fake-consent", "fake-user")
	assert.Error(t, err, "GetAccounts should fail with cancelled context")
}

func TestClient_StructureValidation(t *testing.T) {
	logger := zerolog.Nop()

	// Test that client structure is initialized correctly
	client := NewClient("https://api.example.com", "test-id", "test-secret", logger)

	assert.NotNil(t, client)
	assert.NotNil(t, client.httpClient, "HTTP client should be initialized")
	assert.Equal(t, "https://api.example.com", client.baseURL)
	assert.Equal(t, "test-id", client.clientID)
	assert.Equal(t, "test-secret", client.clientSecret)
}
