package models

import "time"

type BankProvider string

const (
	VBankProvider BankProvider = "vbank"
	SBankProvider BankProvider = "sbank"
	ABankProvider BankProvider = "abank"
)

type ConnectionStatus string

const (
	ConnectionPending ConnectionStatus = "pending"
	ConnectionActive  ConnectionStatus = "active"
	ConnectionExpired ConnectionStatus = "expired"
	ConnectionRevoked ConnectionStatus = "revoked"
)

// BankConnection represents user's connection to a bank
type BankConnection struct {
	ID             int              `json:"id"`
	UserID         int              `json:"user_id"`
	BankProvider   BankProvider     `json:"bank_provider"`
	BankClientID   string           `json:"bank_client_id"` // team-xxx-x
	ConsentID      *string          `json:"consent_id,omitempty"`
	AccessToken    *string          `json:"-"`
	TokenExpiresAt *time.Time       `json:"token_expires_at,omitempty"`
	Status         ConnectionStatus `json:"status"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

// Account represents a bank account from API response
type Account struct {
	AccountID    string       `json:"account_id"`
	BankProvider BankProvider `json:"bank_provider"`
	Currency     string       `json:"currency"`
	AccountType  string       `json:"account_type"`
	Nickname     string       `json:"nickname,omitempty"`
	Servicer     *Servicer    `json:"servicer,omitempty"`
}

// Servicer represents bank info from API
type Servicer struct {
	SchemeName     string `json:"scheme_name"`
	Identification string `json:"identification"`
}

// Balance represents account balance from API
type Balance struct {
	AccountID string  `json:"account_id"`
	Amount    float64 `json:"amount"`
	Currency  string  `json:"currency"`
	Type      string  `json:"type"`
}

// Transaction represents a bank transaction from API response
type Transaction struct {
	TransactionID          string  `json:"transaction_id"`
	AccountID              string  `json:"account_id"`
	Amount                 float64 `json:"amount"`
	Currency               string  `json:"currency"`
	CreditDebitIndicator   string  `json:"credit_debit_indicator"`
	Status                 string  `json:"status"`
	BookingDateTime        string  `json:"booking_date_time"`
	TransactionInformation string  `json:"transaction_information,omitempty"`
}
