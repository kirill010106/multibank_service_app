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
	// Load .env from backend root
	if err := godotenv.Load("../../.env"); err != nil {
		t.Logf("Warning: .env not found, using environment variables")
	}

	baseURL = os.Getenv("VBANK_BASE_URL")
	clientID = os.Getenv("CLIENT_ID")
	clientSecret = os.Getenv("CLIENT_SECRET")

	require.NotEmpty(t, baseURL, "VBANK_BASE_URL must be set")
	require.NotEmpty(t, clientID, "CLIENT_ID must be set")
	require.NotEmpty(t, clientSecret, "CLIENT_SECRET must be set")

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

func TestClient_GetBalances(t *testing.T) {
	baseURL, clientID, clientSecret := loadTestConfig(t)

	log := zerolog.New(os.Stdout).With().Timestamp().Logger()
	client := NewClient(baseURL, clientID, clientSecret, log)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
	require.NotEmpty(t, accounts, "should have at least one account")

	// Step 4: Get balances for first account
	firstAccountID := accounts[0].AccountID
	balances, err := client.GetBalances(ctx, bankToken, bankClientID, consentID, firstAccountID)

	require.NoError(t, err, "GetBalances should succeed")
	assert.NotEmpty(t, balances, "balances should not be empty")

	t.Logf("✅ Fetched %d balances for account %s:", len(balances), firstAccountID)
	for i, bal := range balances {
		t.Logf("  [%d] Amount=%.2f %s, Type=%s",
			i+1, bal.Amount, bal.Currency, bal.Type)
	}

	// Validate balance structure
	for _, bal := range balances {
		assert.NotEmpty(t, bal.AccountID, "account_id should not be empty")
		assert.NotEmpty(t, bal.Currency, "currency should not be empty")
		assert.NotEmpty(t, bal.Type, "type should not be empty")
	}
}

func TestClient_GetTransactions(t *testing.T) {
	baseURL, clientID, clientSecret := loadTestConfig(t)

	log := zerolog.New(os.Stdout).With().Timestamp().Logger()
	client := NewClient(baseURL, clientID, clientSecret, log)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
	require.NotEmpty(t, accounts, "should have at least one account")

	// Step 4: Get transactions for first account
	firstAccountID := accounts[0].AccountID
	transactions, err := client.GetTransactions(ctx, bankToken, bankClientID, consentID, firstAccountID)

	require.NoError(t, err, "GetTransactions should succeed")
	assert.NotEmpty(t, transactions, "transactions should not be empty")

	t.Logf("✅ Fetched %d transactions for account %s:", len(transactions), firstAccountID)
	for i, tx := range transactions {
		t.Logf("  [%d] ID=%s, Amount=%.2f %s, %s, Status=%s, Date=%s",
			i+1, tx.TransactionID, tx.Amount, tx.Currency,
			tx.CreditDebitIndicator, tx.Status, tx.BookingDateTime)
	}

	// Validate transaction structure
	for _, tx := range transactions {
		assert.NotEmpty(t, tx.TransactionID, "transaction_id should not be empty")
		assert.NotEmpty(t, tx.AccountID, "account_id should not be empty")
		assert.NotEmpty(t, tx.Currency, "currency should not be empty")
		assert.NotEmpty(t, tx.CreditDebitIndicator, "credit_debit_indicator should not be empty")
		assert.NotEmpty(t, tx.Status, "status should not be empty")
		assert.NotEmpty(t, tx.BookingDateTime, "booking_date_time should not be empty")
	}
}
