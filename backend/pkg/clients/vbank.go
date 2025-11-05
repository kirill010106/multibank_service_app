package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/kirill010106/multibank_service_app/backend/internal/models"
	"github.com/rs/zerolog"
)

// Client for VBank API
type Client struct {
	baseURL      string
	clientID     string
	clientSecret string
	httpClient   *http.Client
	log          zerolog.Logger
}

func NewClient(baseURL, clientID, clientSecret string, log zerolog.Logger) *Client {
	return &Client{
		baseURL:      baseURL,
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		log:          log,
	}
}

// GetBankToken получает bank token для межбанковых запросов
// POST /auth/bank-token?client_id=team200&client_secret=xxx
func (c *Client) GetBankToken(ctx context.Context) (string, int64, error) {
	const op = "vbank.Client.GetBankToken"

	url := fmt.Sprintf("%s/auth/bank-token?client_id=%s&client_secret=%s", c.baseURL, c.clientID, c.clientSecret)

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return "", 0, fmt.Errorf("%s: create request: %w", op, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("%s: send request: %w", op, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.log.Error().
			Str("op", op).
			Int("status", resp.StatusCode).
			Str("body", string(body)).
			Msg("failed to get bank token")
		return "", 0, fmt.Errorf("%s: unexpected status code: %d", op, resp.StatusCode)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
		TokenType   string `json:"token_type"`
		ClientID    string `json:"client_id"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", 0, fmt.Errorf("%s: decode response: %w", op, err)
	}

	c.log.Info().
		Str("op", op).
		Int64("expires_in", tokenResp.ExpiresIn).
		Msg("bank token obtained")

	return tokenResp.AccessToken, tokenResp.ExpiresIn, nil
}

func (c *Client) CreateConsent(ctx context.Context, bankToken, bankClientID string) (string, error) {
	const op = "vbank.Client.CreateConsent"

	requestBody := map[string]any{
		"client_id": bankClientID,
		"permissions": []string{
			"ReadAccountsDetail",
			"ReadBalances",
			"ReadTransactionsDetail",
		},
		"reason":               "Multibank aggregation",
		"requesting_bank":      c.clientID,
		"requesting_bank_name": "Multibank App",
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("%s: marshal request: %w", op, err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/account-consents/request", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("%s: create request: %w", op, err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bankToken)
	req.Header.Set("X-Requesting-Bank", c.clientID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%s: send request: %w", op, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		c.log.Error().
			Str("op", op).
			Int("status", resp.StatusCode).
			Str("body", string(body)).
			Msg("failed to create consent")
		return "", fmt.Errorf("%s: unexpected status code: %d", op, resp.StatusCode)
	}

	// Read and log raw response for debugging
	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return "", fmt.Errorf("%s: read response body: %w", op, readErr)
	}

	c.log.Debug().
		Str("op", op).
		Str("raw_response", string(bodyBytes)).
		Msg("consent response received")

	var consentResp struct {
		RequestID    string `json:"request_id"`
		ConsentID    string `json:"consent_id"`
		Status       string `json:"status"`
		Message      string `json:"message"`
		CreatedAt    string `json:"created_at"`
		AutoApproved bool   `json:"auto_approved"`
	}

	if err := json.Unmarshal(bodyBytes, &consentResp); err != nil {
		return "", fmt.Errorf("%s: decode response: %w", op, err)
	}

	c.log.Info().
		Str("op", op).
		Str("consent_id", consentResp.ConsentID).
		Str("status", consentResp.Status).
		Bool("auto_approved", consentResp.AutoApproved).
		Msg("consent created")

	return consentResp.ConsentID, nil

}

// GetAccounts получает список счетов клиента
// GET /accounts?client_id=team200-1
func (c *Client) GetAccounts(ctx context.Context, bankToken, bankClientID, consentID string) ([]*models.Account, error) {
	const op = "vbank.Client.GetAccounts"

	url := fmt.Sprintf("%s/accounts?client_id=%s", c.baseURL, bankClientID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: create request: %w", op, err)
	}

	req.Header.Set("Authorization", "Bearer "+bankToken)
	req.Header.Set("X-Requesting-Bank", c.clientID)
	req.Header.Set("X-Consent-Id", consentID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: send request: %w", op, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.log.Error().
			Str("op", op).
			Int("status", resp.StatusCode).
			Str("body", string(body)).
			Msg("failed to get accounts")
		return nil, fmt.Errorf("%s: status %d", op, resp.StatusCode)
	}

	var accountsResp struct {
		Data struct {
			Account []struct {
				AccountID string `json:"AccountId"`
				Currency  string `json:"Currency"`
				Nickname  string `json:"Nickname"`
				Account   []struct {
					SchemeName     string `json:"SchemeName"`
					Identification string `json:"Identification"`
					Name           string `json:"Name"`
				} `json:"Account"`
			} `json:"Account"`
		} `json:"Data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&accountsResp); err != nil {
		return nil, fmt.Errorf("%s: decode response: %w", op, err)
	}

	accounts := make([]*models.Account, 0, len(accountsResp.Data.Account))
	for _, acc := range accountsResp.Data.Account {
		account := &models.Account{
			AccountID:    acc.AccountID,
			BankProvider: models.VBankProvider,
			Currency:     acc.Currency,
			Nickname:     acc.Nickname,
		}

		if len(acc.Account) > 0 {
			account.Servicer = &models.Servicer{
				SchemeName:     acc.Account[0].SchemeName,
				Identification: acc.Account[0].Identification,
			}
		}

		accounts = append(accounts, account)
	}

	c.log.Debug().
		Str("op", op).
		Int("count", len(accounts)).
		Msg("fetched accounts")

	return accounts, nil
}

// GetBalances получает балансы конкретного счета
// GET /accounts/{accountId}/balances?client_id=team200-1
func (c *Client) GetBalances(ctx context.Context, bankToken, bankClientID, consentID, accountID string) ([]*models.Balance, error) {
	const op = "vbank.Client.GetBalances"

	url := fmt.Sprintf("%s/accounts/%s/balances?client_id=%s", c.baseURL, accountID, bankClientID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: create request: %w", op, err)
	}

	req.Header.Set("Authorization", "Bearer "+bankToken)
	req.Header.Set("X-Requesting-Bank", c.clientID)
	req.Header.Set("X-Consent-Id", consentID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: send request: %w", op, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.log.Error().
			Str("op", op).
			Int("status", resp.StatusCode).
			Str("body", string(body)).
			Str("account_id", accountID).
			Msg("failed to get balances")
		return nil, fmt.Errorf("%s: status %d", op, resp.StatusCode)
	}

	var balancesResp struct {
		Data struct {
			Balance []struct {
				AccountID      string `json:"AccountId"`
				CreditDebitInd string `json:"CreditDebitIndicator"`
				Type           string `json:"Type"`
				DateTime       string `json:"DateTime"`
				Amount         struct {
					Amount   string `json:"Amount"`
					Currency string `json:"Currency"`
				} `json:"Amount"`
				CreditLine []struct {
					Included bool `json:"Included"`
					Amount   struct {
						Amount   string `json:"Amount"`
						Currency string `json:"Currency"`
					} `json:"Amount"`
				} `json:"CreditLine,omitempty"`
			} `json:"Balance"`
		} `json:"Data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&balancesResp); err != nil {
		return nil, fmt.Errorf("%s: decode response: %w", op, err)
	}

	balances := make([]*models.Balance, 0, len(balancesResp.Data.Balance))
	for _, bal := range balancesResp.Data.Balance {
		// Parse amount string to float64
		var amount float64
		if _, err := fmt.Sscanf(bal.Amount.Amount, "%f", &amount); err != nil {
			c.log.Warn().
				Str("op", op).
				Str("amount", bal.Amount.Amount).
				Msg("failed to parse amount, skipping balance")
			continue
		}

		balance := &models.Balance{
			AccountID: bal.AccountID,
			Amount:    amount,
			Currency:  bal.Amount.Currency,
			Type:      bal.Type,
		}

		balances = append(balances, balance)
	}

	c.log.Debug().
		Str("op", op).
		Str("account_id", accountID).
		Int("count", len(balances)).
		Msg("fetched balances")

	return balances, nil
}

// GetTransactions получает список транзакций конкретного счета
// GET /accounts/{accountId}/transactions?client_id=team200-1
func (c *Client) GetTransactions(ctx context.Context, bankToken, bankClientID, consentID, accountID string) ([]*models.Transaction, error) {
	const op = "vbank.Client.GetTransactions"

	url := fmt.Sprintf("%s/accounts/%s/transactions?client_id=%s", c.baseURL, accountID, bankClientID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: create request: %w", op, err)
	}

	req.Header.Set("Authorization", "Bearer "+bankToken)
	req.Header.Set("X-Requesting-Bank", c.clientID)
	req.Header.Set("X-Consent-Id", consentID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: send request: %w", op, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.log.Error().
			Str("op", op).
			Int("status", resp.StatusCode).
			Str("body", string(body)).
			Str("account_id", accountID).
			Msg("failed to get transactions")
		return nil, fmt.Errorf("%s: status %d", op, resp.StatusCode)
	}

	var transactionsResp struct {
		Data struct {
			Transaction []struct {
				AccountID            string `json:"AccountId"`
				TransactionID        string `json:"TransactionId"`
				CreditDebitIndicator string `json:"CreditDebitIndicator"`
				Status               string `json:"Status"`
				BookingDateTime      string `json:"BookingDateTime"`
				Amount               struct {
					Amount   string `json:"Amount"`
					Currency string `json:"Currency"`
				} `json:"Amount"`
				TransactionInformation string `json:"TransactionInformation,omitempty"`
			} `json:"Transaction"`
		} `json:"Data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&transactionsResp); err != nil {
		return nil, fmt.Errorf("%s: decode response: %w", op, err)
	}

	transactions := make([]*models.Transaction, 0, len(transactionsResp.Data.Transaction))
	for _, tx := range transactionsResp.Data.Transaction {
		// Parse amount string to float64
		var amount float64
		if _, err := fmt.Sscanf(tx.Amount.Amount, "%f", &amount); err != nil {
			c.log.Warn().
				Str("op", op).
				Str("amount", tx.Amount.Amount).
				Str("transaction_id", tx.TransactionID).
				Msg("failed to parse amount, skipping transaction")
			continue
		}

		transaction := &models.Transaction{
			TransactionID:          tx.TransactionID,
			AccountID:              tx.AccountID,
			Amount:                 amount,
			Currency:               tx.Amount.Currency,
			CreditDebitIndicator:   tx.CreditDebitIndicator,
			Status:                 tx.Status,
			BookingDateTime:        tx.BookingDateTime,
			TransactionInformation: tx.TransactionInformation,
		}

		transactions = append(transactions, transaction)
	}

	c.log.Debug().
		Str("op", op).
		Str("account_id", accountID).
		Int("count", len(transactions)).
		Msg("fetched transactions")

	return transactions, nil
}

func (c *Client) GetBankProvider() models.BankProvider {
	return models.VBankProvider
}
