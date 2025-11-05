package bank

import (
	"context"
	"fmt"
	"time"

	"github.com/kirill010106/multibank_service_app/backend/internal/models"
	"github.com/kirill010106/multibank_service_app/backend/internal/repository"
	"github.com/kirill010106/multibank_service_app/backend/pkg/clients"
	"github.com/rs/zerolog"
)

type Service struct {
	connRepo    repository.BankConnectionRepository
	bankClients map[models.BankProvider]clients.BankClient
	log         zerolog.Logger
}

func NewService(
	connRepo repository.BankConnectionRepository,
	bankClients map[models.BankProvider]clients.BankClient,
	log zerolog.Logger,
) *Service {
	return &Service{
		connRepo:    connRepo,
		bankClients: bankClients,
		log:         log,
	}
}

type ConnectBankRequest struct {
	UserID       int                 `json:"user_id" valid:"required"`
	BankProvider models.BankProvider `json:"bank_provider" valid:"required"`
	BankClientID string              `json:"bank_client_id" valid:"required"`
}

type ConnectBankResponse struct {
	Connection *models.BankConnection `json:"connection"`
	ConsentID  string                 `json:"consent_id"`
	Status     string                 `json:"status"`
	Message    string                 `json:"message"`
}

// ConnectBank creates a new bank connection for user
// Steps:
// 1. Get bank token from bank API
// 2. Create consent for accessing user's accounts
// 3. Save connection to database

func (s *Service) ConnectBank(ctx context.Context, req *ConnectBankRequest) (*ConnectBankResponse, error) {
	const op = "bank.Service.ConnectBank"

	s.log.Info().
		Str("op", op).
		Int("user_id", req.UserID).
		Str("bank_provider", string(req.BankProvider)).
		Str("bank_client_id", req.BankClientID).
		Msg("Connecting bank")

	bankClient, ok := s.bankClients[req.BankProvider]

	if !ok {
		return nil, fmt.Errorf("%s: unsupported bank provider %s", op, req.BankProvider)
	}

	// Step 1: Get bank token

	s.log.Debug().Str("op", op).Msg("Getting bank token")
	bankToken, expiresIn, err := bankClient.GetBankToken(ctx)

	if err != nil {
		s.log.Error().Err(err).Str("op", op).Msg("failed to get bank token")
		return nil, fmt.Errorf("%s: failed to get bank token: %w", op, err)
	}

	tokenExpiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)

	s.log.Debug().Str("op", op).Msg("Bank token obtained")

	// Step 2: Create consent for accessing user's accounts
	s.log.Debug().Str("op", op).Msg("Creating consent for accessing user's accounts")
	consentID, err := bankClient.CreateConsent(ctx, bankToken, req.BankClientID)
	if err != nil {
		s.log.Error().Err(err).Str("op", op).Msg("failed to create consent")
		return nil, fmt.Errorf("%s: create consent: %w", op, err)
	}

	s.log.Debug().Str("op", op).Msg("Consent created")

	// Step 3: Save connection to database
	s.log.Debug().Str("op", op).Msg("Saving bank connection to database")
	connection := &models.BankConnection{
		UserID:         req.UserID,
		BankProvider:   req.BankProvider,
		BankClientID:   req.BankClientID,
		ConsentID:      &consentID,
		AccessToken:    &bankToken,
		TokenExpiresAt: &tokenExpiresAt,
		Status:         models.ConnectionActive,
	}

	if err := s.connRepo.Create(ctx, connection); err != nil {
		s.log.Error().Err(err).Str("op", op).Msg("failed to save bank connection to database")
		return nil, fmt.Errorf("%s: save connection: %w", op, err)
	}

	s.log.Info().
		Str("op", op).
		Int("connection_id", connection.ID).
		Str("consent_id", consentID).
		Msg("Bank connected successfully")

	return &ConnectBankResponse{
		Connection: connection,
		ConsentID:  consentID,
		Status:     "success",
		Message:    fmt.Sprintf("Connected successfully to %s", req.BankProvider),
	}, nil

}

func (s *Service) GetConnections(ctx context.Context, userID int) ([]*models.BankConnection, error) {
	const op = "bank.Service.GetConnections"

	s.log.Debug().
		Str("op", op).
		Int("user_id", userID).
		Msg("Fetching bank connections for user")

	connections, err := s.connRepo.FindAllByUser(ctx, userID)
	if err != nil {
		s.log.Error().Err(err).Str("op", op).Msg("failed to fetch connections")
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	s.log.Debug().
		Str("op", op).
		Int("count", len(connections)).
		Msg("Bank connections fetched successfully")

	return connections, nil

}

type GetAccountsRequest struct {
	UserID       int                 `json:"user_id" validate:"required"`
	BankProvider models.BankProvider `json:"bank_provider" validate:"required"`
}

// GetAccounts fetches accounts from a connected bank
// Steps:
// 1. Find active connection
// 2. Check if token is expired (refresh if needed)
// 3. Fetch accounts from bank API
func (s *Service) GetAccounts(ctx context.Context, req *GetAccountsRequest) ([]*models.Account, error) {
	const op = "bank.Service.GetAccounts"

	s.log.Debug().
		Str("op", op).
		Int("user_id", req.UserID).
		Str("bank_provider", string(req.BankProvider)).
		Msg("Fetching bank accounts")

	// Step 1: Find active connection
	connection, err := s.connRepo.FindByUserAndBank(ctx, req.UserID, req.BankProvider)
	if err != nil {
		s.log.Error().Err(err).Str("op", op).Msg("connection not found")
		return nil, fmt.Errorf("%s: connection not found: %w", op, err)
	}

	if connection.Status != models.ConnectionActive {
		return nil, fmt.Errorf("%s: connection is not active (status: %s)", op, connection.Status)
	}

	// Step 2: Check if token is expired (refresh if needed)
	if connection.TokenExpiresAt != nil && time.Now().After(*connection.TokenExpiresAt) {
		s.log.Warn().
			Str("op", op).
			Int("connection_id", connection.ID).
			Msg("access token expired, refreshing")

		if err := s.refreshToken(ctx, connection); err != nil {
			s.log.Error().Err(err).Str("op", op).Msg("failed to refresh access token")
			return nil, fmt.Errorf("%s: failed to refresh access token: %w", op, err)
		}
	}

	// Step 3: Get bank client
	bankClient, ok := s.bankClients[req.BankProvider]
	if !ok {
		return nil, fmt.Errorf("%s: unsupported bank provider %s", op, req.BankProvider)
	}

	if connection.AccessToken == nil {
		return nil, fmt.Errorf("%s: missing access token", op)
	}
	if connection.ConsentID == nil {
		return nil, fmt.Errorf("%s: missing consent ID", op)
	}

	// Step 4: Fetch accounts from bank API
	accounts, err := bankClient.GetAccounts(ctx,
		*connection.AccessToken,
		connection.BankClientID,
		*connection.ConsentID,
	)
	if err != nil {
		s.log.Error().Err(err).Str("op", op).Msg("failed to fetch accounts from bank API")
		return nil, fmt.Errorf("%s: failed to fetch accounts: %w", op, err)
	}

	s.log.Info().
		Str("op", op).
		Int("user_id", req.UserID).
		Str("bank_provider", string(req.BankProvider)).
		Int("accounts_count", len(accounts)).
		Msg("Bank accounts fetched successfully")
	return accounts, nil
}

// refreshToken refreshes expired bank token
func (s *Service) refreshToken(ctx context.Context, connection *models.BankConnection) error {
	const op = "bank.Service.refreshToken"

	bankClient, ok := s.bankClients[connection.BankProvider]

	if !ok {
		return fmt.Errorf("%s: unsupported bank provider %s", op, connection.BankProvider)
	}

	newToken, expiresIn, err := bankClient.GetBankToken(ctx)
	if err != nil {
		return fmt.Errorf("%s: failed to get bank token: %w", op, err)
	}

	newExpiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)

	if err := s.connRepo.UpdateToken(ctx, connection.ID, newToken, newExpiresAt); err != nil {
		return fmt.Errorf("%s: failed to update token in database: %w", op, err)
	}

	// Update in-memory connection object
	connection.AccessToken = &newToken
	connection.TokenExpiresAt = &newExpiresAt

	s.log.Info().
		Str("op", op).
		Int("connection_id", connection.ID).
		Time("new_expires_at", newExpiresAt).
		Msg("Access token refreshed successfully")

	return nil
}

// DisconnectBank revokes a bank connection
func (s *Service) DisconnectBank(ctx context.Context, userID int, bankProvider models.BankProvider) error {
	const op = "bank.Service.DisconnectBank"

	s.log.Info().
		Str("op", op).
		Int("user_id", userID).
		Str("bank_provider", string(bankProvider)).
		Msg("Disconnecting bank")

	connection, err := s.connRepo.FindByUserAndBank(ctx, userID, bankProvider)
	if err != nil {
		return fmt.Errorf("%s: connection not found: %w", op, err)
	}

	if err := s.connRepo.Delete(ctx, connection.ID); err != nil {
		s.log.Error().Err(err).Str("op", op).Msg("failed to delete connection")
		return fmt.Errorf("%s: delete connection: %w", op, err)
	}

	s.log.Info().
		Str("op", op).
		Int("connection_id", connection.ID).
		Msg("Bank disconnected successfully")

	return nil
}

// GetDashboard aggregates data from all connected banks
func (s *Service) GetDashboard(ctx context.Context, userID int) (*models.DashboardResponse, error) {
	const op = "bank.Service.GetDashboard"

	s.log.Info().
		Str("op", op).
		Int("user_id", userID).
		Msg("fetching dashboard data")

	// Step 1: Get all user's bank connections
	connections, err := s.connRepo.FindAllByUser(ctx, userID)
	if err != nil {
		s.log.Error().Err(err).Str("op", op).Msg("failed to fetch connections")
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	dashboard := &models.DashboardResponse{
		UserID:     userID,
		TotalBanks: len(connections),
		Banks:      make([]models.BankDashboardInfo, 0, len(connections)),
	}

	// Step 2: Fetch accounts from each bank (parallel safe - iterate sequentially)
	for _, conn := range connections {
		bankInfo := s.fetchBankAccounts(ctx, conn)
		dashboard.Banks = append(dashboard.Banks, bankInfo)

		// Count active banks (no error)
		if bankInfo.Error == "" {
			dashboard.ActiveBanks++
			dashboard.TotalAccounts += bankInfo.AccountsCount
		}
	}

	s.log.Info().
		Str("op", op).
		Int("user_id", userID).
		Int("total_banks", dashboard.TotalBanks).
		Int("active_banks", dashboard.ActiveBanks).
		Int("total_accounts", dashboard.TotalAccounts).
		Msg("dashboard data fetched")

	return dashboard, nil
}

// fetchBankAccounts fetches accounts for a single bank connection
// If bank is unavailable, returns error in BankDashboardInfo (not failing entire dashboard)
func (s *Service) fetchBankAccounts(ctx context.Context, conn *models.BankConnection) models.BankDashboardInfo {
	const op = "bank.Service.fetchBankAccounts"

	bankInfo := models.BankDashboardInfo{
		Provider: conn.BankProvider,
		Status:   string(conn.Status),
	}

	// Check connection status
	if conn.Status != models.ConnectionActive {
		bankInfo.Error = "connection not active"
		s.log.Warn().
			Str("op", op).
			Int("connection_id", conn.ID).
			Str("provider", string(conn.BankProvider)).
			Str("status", string(conn.Status)).
			Msg("skipping inactive connection")
		return bankInfo
	}

	// Refresh token if expired
	if conn.TokenExpiresAt != nil && time.Now().After(*conn.TokenExpiresAt) {
		s.log.Debug().
			Str("op", op).
			Int("connection_id", conn.ID).
			Msg("token expired, refreshing")

		if err := s.refreshToken(ctx, conn); err != nil {
			bankInfo.Error = fmt.Sprintf("token refresh failed: %v", err)
			s.log.Error().
				Err(err).
				Str("op", op).
				Int("connection_id", conn.ID).
				Msg("failed to refresh token")
			return bankInfo
		}
	}

	// Get bank client
	bankClient, ok := s.bankClients[conn.BankProvider]
	if !ok {
		bankInfo.Error = "unsupported bank provider"
		s.log.Error().
			Str("op", op).
			Str("provider", string(conn.BankProvider)).
			Msg("bank client not found")
		return bankInfo
	}

	// Validate required fields
	if conn.AccessToken == nil || conn.ConsentID == nil {
		bankInfo.Error = "missing credentials"
		s.log.Error().
			Str("op", op).
			Int("connection_id", conn.ID).
			Msg("connection missing token or consent")
		return bankInfo
	}

	// Fetch accounts from bank API
	accounts, err := bankClient.GetAccounts(
		ctx,
		*conn.AccessToken,
		conn.BankClientID,
		*conn.ConsentID,
	)
	if err != nil {
		bankInfo.Error = fmt.Sprintf("bank api error: %v", err)
		s.log.Error().
			Err(err).
			Str("op", op).
			Int("connection_id", conn.ID).
			Str("provider", string(conn.BankProvider)).
			Msg("failed to fetch accounts from bank")
		return bankInfo
	}

	// Success - populate accounts
	bankInfo.Accounts = accounts
	bankInfo.AccountsCount = len(accounts)

	s.log.Debug().
		Str("op", op).
		Int("connection_id", conn.ID).
		Str("provider", string(conn.BankProvider)).
		Int("accounts_count", len(accounts)).
		Msg("accounts fetched successfully")

	return bankInfo
}

type GetBalancesRequest struct {
	UserID       int                 `json:"user_id" validate:"required"`
	BankProvider models.BankProvider `json:"bank_provider" validate:"required"`
	AccountID    string              `json:"account_id" validate:"required"`
}

// GetBalances fetches balances for a specific account
// Steps:
// 1. Find active connection
// 2. Check if token is expired (refresh if needed)
// 3. Fetch balances from bank API
func (s *Service) GetBalances(ctx context.Context, req *GetBalancesRequest) ([]*models.Balance, error) {
	const op = "bank.Service.GetBalances"

	s.log.Debug().
		Str("op", op).
		Int("user_id", req.UserID).
		Str("bank_provider", string(req.BankProvider)).
		Str("account_id", req.AccountID).
		Msg("fetching account balances")

	// Step 1: Find active connection
	connection, err := s.connRepo.FindByUserAndBank(ctx, req.UserID, req.BankProvider)
	if err != nil {
		s.log.Error().Err(err).Str("op", op).Msg("connection not found")
		return nil, fmt.Errorf("%s: connection not found: %w", op, err)
	}

	if connection.Status != models.ConnectionActive {
		return nil, fmt.Errorf("%s: connection is not active (status: %s)", op, connection.Status)
	}

	// Step 2: Check if token is expired (refresh if needed)
	if connection.TokenExpiresAt != nil && time.Now().After(*connection.TokenExpiresAt) {
		s.log.Warn().
			Str("op", op).
			Int("connection_id", connection.ID).
			Msg("access token expired, refreshing")

		if err := s.refreshToken(ctx, connection); err != nil {
			s.log.Error().Err(err).Str("op", op).Msg("failed to refresh access token")
			return nil, fmt.Errorf("%s: failed to refresh access token: %w", op, err)
		}
	}

	// Step 3: Get bank client
	bankClient, ok := s.bankClients[req.BankProvider]
	if !ok {
		return nil, fmt.Errorf("%s: unsupported bank provider %s", op, req.BankProvider)
	}

	if connection.AccessToken == nil {
		return nil, fmt.Errorf("%s: missing access token", op)
	}
	if connection.ConsentID == nil {
		return nil, fmt.Errorf("%s: missing consent ID", op)
	}

	// Step 4: Fetch balances from bank API
	balances, err := bankClient.GetBalances(ctx,
		*connection.AccessToken,
		connection.BankClientID,
		*connection.ConsentID,
		req.AccountID,
	)
	if err != nil {
		s.log.Error().Err(err).Str("op", op).Msg("failed to fetch balances from bank API")
		return nil, fmt.Errorf("%s: failed to fetch balances: %w", op, err)
	}

	s.log.Info().
		Str("op", op).
		Int("user_id", req.UserID).
		Str("bank_provider", string(req.BankProvider)).
		Str("account_id", req.AccountID).
		Int("balances_count", len(balances)).
		Msg("account balances fetched successfully")

	return balances, nil
}

type GetTransactionsRequest struct {
	UserID       int                 `json:"user_id" validate:"required"`
	BankProvider models.BankProvider `json:"bank_provider" validate:"required"`
	AccountID    string              `json:"account_id" validate:"required"`
}

// GetTransactions fetches transactions for a specific account
// Steps:
// 1. Find active connection
// 2. Check if token is expired (refresh if needed)
// 3. Fetch transactions from bank API
func (s *Service) GetTransactions(ctx context.Context, req *GetTransactionsRequest) ([]*models.Transaction, error) {
	const op = "bank.Service.GetTransactions"

	s.log.Debug().
		Str("op", op).
		Int("user_id", req.UserID).
		Str("bank_provider", string(req.BankProvider)).
		Str("account_id", req.AccountID).
		Msg("fetching account transactions")

	// Step 1: Find active connection
	connection, err := s.connRepo.FindByUserAndBank(ctx, req.UserID, req.BankProvider)
	if err != nil {
		s.log.Error().Err(err).Str("op", op).Msg("connection not found")
		return nil, fmt.Errorf("%s: connection not found: %w", op, err)
	}

	if connection.Status != models.ConnectionActive {
		return nil, fmt.Errorf("%s: connection is not active (status: %s)", op, connection.Status)
	}

	// Step 2: Check if token is expired (refresh if needed)
	if connection.TokenExpiresAt != nil && time.Now().After(*connection.TokenExpiresAt) {
		s.log.Warn().
			Str("op", op).
			Int("connection_id", connection.ID).
			Msg("access token expired, refreshing")

		if err := s.refreshToken(ctx, connection); err != nil {
			s.log.Error().Err(err).Str("op", op).Msg("failed to refresh access token")
			return nil, fmt.Errorf("%s: failed to refresh access token: %w", op, err)
		}
	}

	// Step 3: Get bank client
	bankClient, ok := s.bankClients[req.BankProvider]
	if !ok {
		return nil, fmt.Errorf("%s: unsupported bank provider %s", op, req.BankProvider)
	}

	if connection.AccessToken == nil {
		return nil, fmt.Errorf("%s: missing access token", op)
	}
	if connection.ConsentID == nil {
		return nil, fmt.Errorf("%s: missing consent ID", op)
	}

	// Step 4: Fetch transactions from bank API
	transactions, err := bankClient.GetTransactions(ctx,
		*connection.AccessToken,
		connection.BankClientID,
		*connection.ConsentID,
		req.AccountID,
	)
	if err != nil {
		s.log.Error().Err(err).Str("op", op).Msg("failed to fetch transactions from bank API")
		return nil, fmt.Errorf("%s: failed to fetch transactions: %w", op, err)
	}

	s.log.Info().
		Str("op", op).
		Int("user_id", req.UserID).
		Str("bank_provider", string(req.BankProvider)).
		Str("account_id", req.AccountID).
		Int("transactions_count", len(transactions)).
		Msg("account transactions fetched successfully")

	return transactions, nil
}
