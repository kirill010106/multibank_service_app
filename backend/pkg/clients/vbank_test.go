//go:build integration
// +build integration

package clients

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loadTestConfig loads .env and returns test credentials
func loadTestConfig(t *testing.T) (baseURL, clientID, clientSecret string) {
	t.Helper()

	// Load .env from backend root (only works locally)
	if err := godotenv.Load("../../.env"); err != nil {
		t.Logf("Warning: .env not found, using environment variables")
	}

	baseURL = os.Getenv("VBANK_BASE_URL")
	clientID = os.Getenv("CLIENT_ID")
	clientSecret = os.Getenv("CLIENT_SECRET")

	// Skip test if credentials not available (instead of failing)
	if baseURL == "" || clientID == "" || clientSecret == "" {
		t.Skip("Skipping integration test: VBANK credentials not set (VBANK_BASE_URL, CLIENT_ID, CLIENT_SECRET required)")
	}

	return
}

func TestClient_GetBankToken(t *testing.T) {
	baseURL, clientID, clientSecret := loadTestConfig(t)

	log := zerolog.New(os.Stdout).With().Timestamp().Logger()
	client := NewClient(baseURL, clientID, clientSecret, log)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	token, expiresIn, err := client.GetBankToken(ctx)

	require.NoError(t, err, "GetBankToken should succeed")
	assert.NotEmpty(t, token, "token should not be empty")
	assert.Greater(t, expiresIn, int64(0), "expiresIn should be positive")

	t.Logf("✅ Token obtained: %d characters, expires in %d seconds", len(token), expiresIn)
}

func TestClient_CreateConsent(t *testing.T) {
	baseURL, clientID, clientSecret := loadTestConfig(t)

	log := zerolog.New(os.Stdout).With().Timestamp().Logger()
	client := NewClient(baseURL, clientID, clientSecret, log)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Step 1: Get bank token
	bankToken, _, err := client.GetBankToken(ctx)
	require.NoError(t, err, "GetBankToken should succeed")

	// Step 2: Create consent for test client
	bankClientID := clientID + "-1" // team200-1
	consentID, err := client.CreateConsent(ctx, bankToken, bankClientID)

	require.NoError(t, err, "CreateConsent should succeed")
	assert.NotEmpty(t, consentID, "consent_id should not be empty")

	t.Logf("✅ Consent created: %s for client %s", consentID, bankClientID)
}

func TestClient_GetAccounts(t *testing.T) {
	baseURL, clientID, clientSecret := loadTestConfig(t)

	log := zerolog.New(os.Stdout).With().Timestamp().Logger()
	client := NewClient(baseURL, clientID, clientSecret, log)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Step 1: Get bank token
	bankToken, _, err := client.GetBankToken(ctx)
	require.NoError(t, err, "GetBankToken should succeed")

	// Step 2: Create consent
	bankClientID := clientID + "-1"
	consentID, err := client.CreateConsent(ctx, bankToken, bankClientID)
	require.NoError(t, err, "CreateConsent should succeed")

	// Step 3: Get accounts
	accounts, err := client.GetAccounts(ctx, bankToken, bankClientID, consentID)

	require.NoError(t, err, "GetAccounts should succeed")
	assert.NotEmpty(t, accounts, "accounts should not be empty")

	t.Logf("✅ Fetched %d accounts:", len(accounts))
	for i, acc := range accounts {
		t.Logf("  [%d] ID=%s, Currency=%s, Nickname=%s",
			i+1, acc.AccountID, acc.Currency, acc.Nickname)
	}
}

func TestClient_FullFlow(t *testing.T) {
	baseURL, clientID, clientSecret := loadTestConfig(t)

	log := zerolog.New(os.Stdout).With().Timestamp().Logger()
	client := NewClient(baseURL, clientID, clientSecret, log)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Run("Step 1: GetBankToken", func(t *testing.T) {
		token, expiresIn, err := client.GetBankToken(ctx)
		require.NoError(t, err)
		assert.NotEmpty(t, token)
		assert.Greater(t, expiresIn, int64(0))

		// Save for next steps
		ctx = context.WithValue(ctx, "bankToken", token)
	})

	bankToken := ctx.Value("bankToken").(string)

	t.Run("Step 2: CreateConsent", func(t *testing.T) {
		bankClientID := clientID + "-1"
		consentID, err := client.CreateConsent(ctx, bankToken, bankClientID)
		require.NoError(t, err)
		assert.NotEmpty(t, consentID)

		ctx = context.WithValue(ctx, "consentID", consentID)
		ctx = context.WithValue(ctx, "bankClientID", bankClientID)
	})

	consentID := ctx.Value("consentID").(string)
	bankClientID := ctx.Value("bankClientID").(string)

	t.Run("Step 3: GetAccounts", func(t *testing.T) {
		accounts, err := client.GetAccounts(ctx, bankToken, bankClientID, consentID)
		require.NoError(t, err)
		assert.NotEmpty(t, accounts)

		// Validate account structure
		for _, acc := range accounts {
			assert.NotEmpty(t, acc.AccountID)
			assert.NotEmpty(t, acc.Currency)
			assert.Equal(t, "vbank", string(acc.BankProvider))
		}
	})

	t.Logf("✅ Full flow completed successfully")
}
