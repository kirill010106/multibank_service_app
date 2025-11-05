package handlers

import (
	"context"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/kirill010106/multibank_service_app/backend/internal/models"
	"github.com/kirill010106/multibank_service_app/backend/internal/services/bank"
	"github.com/rs/zerolog"
)

// BankServiceInterface defines methods that Bank Service must implement
type BankServiceInterface interface {
	ConnectBank(ctx context.Context, req *bank.ConnectBankRequest) (*bank.ConnectBankResponse, error)
	GetConnections(ctx context.Context, userID int) ([]*models.BankConnection, error)
	GetAccounts(ctx context.Context, req *bank.GetAccountsRequest) ([]*models.Account, error)
	GetBalances(ctx context.Context, req *bank.GetBalancesRequest) ([]*models.Balance, error)
	GetTransactions(ctx context.Context, req *bank.GetTransactionsRequest) ([]*models.Transaction, error)
	DisconnectBank(ctx context.Context, userID int, provider models.BankProvider) error
	GetDashboard(ctx context.Context, userID int) (*models.DashboardResponse, error)
}

type BankHandler struct {
	bankService BankServiceInterface
	log         zerolog.Logger
}

func NewBankHandler(bankService BankServiceInterface, log zerolog.Logger) *BankHandler {
	return &BankHandler{
		bankService: bankService,
		log:         log,
	}
}

type ConnectBankRequest struct {
	BankProvider string `json:"bank_provider" validate:"required,oneof=vbank sbank abank"`
	BankClientID string `json:"bank_client_id" validate:"required"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

func (h *BankHandler) getUserID(c *fiber.Ctx) (int, error) {
	const op = "handlers.BankHandler.getUserID"

	userID := c.Locals("user_id")
	if userID == nil {
		h.log.Error().Str("op", op).Msg("user_id not found in context")
		return 0, fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}

	uid, ok := userID.(int)
	if !ok {
		h.log.Error().Str("op", op).Msg("user_id is not int")
		return 0, fiber.NewError(fiber.StatusInternalServerError, "invalid user_id type")
	}
	return uid, nil
}

func (h *BankHandler) sendError(c *fiber.Ctx, statusCode int, err error, message string) error {
	h.log.Error().
		Err(err).
		Int("status", statusCode).
		Str("path", c.Path()).
		Msg(message)

	return c.Status(statusCode).JSON(ErrorResponse{
		Error:   err.Error(),
		Message: message,
	})
}

// ConnectBank godoc
// @Summary      Connect bank account
// @Description  Create connection to a bank using bank client ID
// @Tags         banks
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body ConnectBankRequest true "Bank connection details"
// @Success      201 {object} bank.ConnectBankResponse "Bank connected successfully"
// @Failure      400 {object} ErrorResponse "Invalid request or validation error"
// @Failure      401 {object} ErrorResponse "Unauthorized"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /banks/connect [post]
func (h *BankHandler) ConnectBank(c *fiber.Ctx) error {
	const op = "handlers.BankHandler.ConnectBank"

	var req ConnectBankRequest
	if err := c.BodyParser(&req); err != nil {
		return h.sendError(c, fiber.StatusBadRequest, err, "invalid request body")
	}

	validate := validator.New()
	if err := validate.Struct(&req); err != nil {
		return h.sendError(c, fiber.StatusBadRequest, err, "validation failed")
	}

	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	provider := models.BankProvider(req.BankProvider)

	resp, err := h.bankService.ConnectBank(c.Context(), &bank.ConnectBankRequest{
		UserID:       userID,
		BankProvider: provider,
		BankClientID: req.BankClientID,
	})

	if err != nil {
		return h.sendError(c, fiber.StatusInternalServerError, err, "Failed to connect bank")
	}

	h.log.Info().
		Str("op", op).
		Int("user_id", userID).
		Str("bank_provider", req.BankProvider).
		Msg("Bank connected successfully")

	return c.Status(fiber.StatusCreated).JSON(resp)
}

// GetConnections godoc
// @Summary      Get user's bank connections
// @Description  Retrieve all bank connections for authenticated user
// @Tags         banks
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} map[string]interface{} "List of connections"
// @Failure      401 {object} ErrorResponse "Unauthorized"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /banks [get]
func (h *BankHandler) GetConnections(c *fiber.Ctx) error {
	const op = "handlers.BankHandler.GetConnections"

	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}
	connections, err := h.bankService.GetConnections(c.Context(), userID)
	if err != nil {
		return h.sendError(c, fiber.StatusInternalServerError, err, "Failed to fetch bank connections")
	}

	h.log.Debug().
		Str("op", op).
		Int("user_id", userID).
		Int("connections_count", len(connections)).
		Msg("Fetched bank connections successfully")

	return c.JSON(fiber.Map{
		"connections": connections,
		"count":       len(connections),
	})
}

// GetAccounts godoc
// @Summary      Get bank accounts
// @Description  Retrieve all accounts from specified bank provider
// @Tags         banks
// @Produce      json
// @Security     BearerAuth
// @Param        provider path string true "Bank provider" Enums(vbank, sbank, abank)
// @Success      200 {object} map[string]interface{} "List of accounts"
// @Failure      400 {object} ErrorResponse "Invalid bank provider"
// @Failure      401 {object} ErrorResponse "Unauthorized"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /banks/{provider}/accounts [get]
func (h *BankHandler) GetAccounts(c *fiber.Ctx) error {
	const op = "handlers.BankHandler.GetAccounts"

	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	// Получаем provider из URL (/api/v1/banks/:provider/accounts)
	providerStr := c.Params("provider")
	if providerStr == "" {
		return h.sendError(c, fiber.StatusBadRequest,
			fiber.NewError(fiber.StatusBadRequest, "missing provider"),
			"bank provider is required")
	}

	provider := models.BankProvider(providerStr)
	if provider != models.VBankProvider && provider != models.SBankProvider && provider != models.ABankProvider {
		return h.sendError(c, fiber.StatusBadRequest,
			fiber.NewError(fiber.StatusBadRequest, "invalid provider"),
			"unsupported bank provider")
	}

	accounts, err := h.bankService.GetAccounts(c.Context(), &bank.GetAccountsRequest{
		UserID:       userID,
		BankProvider: provider,
	})

	if err != nil {
		return h.sendError(c, fiber.StatusInternalServerError, err, "Failed to fetch accounts")
	}

	h.log.Info().
		Str("op", op).
		Int("user_id", userID).
		Str("bank_provider", providerStr).
		Int("accounts_count", len(accounts)).
		Msg("accounts fetched")

	return c.JSON(fiber.Map{
		"bank_provider": providerStr,
		"accounts":      accounts,
		"count":         len(accounts),
	})
}

// DisconnectBank godoc
// @Summary      Disconnect bank
// @Description  Remove bank connection for authenticated user
// @Tags         banks
// @Security     BearerAuth
// @Param        provider path string true "Bank provider" Enums(vbank, sbank, abank)
// @Success      204 "Bank disconnected successfully"
// @Failure      400 {object} ErrorResponse "Invalid bank provider"
// @Failure      401 {object} ErrorResponse "Unauthorized"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /banks/{provider} [delete]
func (h *BankHandler) DisconnectBank(c *fiber.Ctx) error {
	const op = "handlers.BankHandler.DisconnectBank"

	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	providerStr := c.Params("provider")
	if providerStr == "" {
		return h.sendError(c, fiber.StatusBadRequest,
			fiber.NewError(fiber.StatusBadRequest, "missing provider"),
			"bank provider is required")
	}

	provider := models.BankProvider(providerStr)
	if provider != models.VBankProvider && provider != models.SBankProvider && provider != models.ABankProvider {
		return h.sendError(c, fiber.StatusBadRequest,
			fiber.NewError(fiber.StatusBadRequest, "invalid provider"),
			"Unsupported bank provider")
	}

	if err := h.bankService.DisconnectBank(c.Context(), userID, provider); err != nil {
		return h.sendError(c, fiber.StatusInternalServerError, err, "Failed to disconnect bank")
	}

	h.log.Info().
		Str("op", op).
		Int("user_id", userID).
		Str("bank_provider", providerStr).
		Msg("bank disconnected")

	// 204 No Content - успешное удаление без body
	return c.SendStatus(fiber.StatusNoContent)
}

// GetBalances godoc
// @Summary      Get account balances
// @Description  Retrieve balances for a specific account from a connected bank
// @Tags         banks
// @Security     BearerAuth
// @Param        provider path string true "Bank provider" Enums(vbank, sbank, abank)
// @Param        accountId path string true "Account ID"
// @Produce      json
// @Success      200 {object} object{bank_provider=string,account_id=string,balances=[]models.Balance,count=int}
// @Failure      400 {object} ErrorResponse "Invalid parameters"
// @Failure      401 {object} ErrorResponse "Unauthorized"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /banks/{provider}/accounts/{accountId}/balances [get]
func (h *BankHandler) GetBalances(c *fiber.Ctx) error {
	const op = "handlers.BankHandler.GetBalances"

	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	providerStr := c.Params("provider")
	if providerStr == "" {
		return h.sendError(c, fiber.StatusBadRequest,
			fiber.NewError(fiber.StatusBadRequest, "missing provider"),
			"bank provider is required")
	}

	accountID := c.Params("accountId")
	if accountID == "" {
		return h.sendError(c, fiber.StatusBadRequest,
			fiber.NewError(fiber.StatusBadRequest, "missing account ID"),
			"account ID is required")
	}

	provider := models.BankProvider(providerStr)
	if provider != models.VBankProvider && provider != models.SBankProvider && provider != models.ABankProvider {
		return h.sendError(c, fiber.StatusBadRequest,
			fiber.NewError(fiber.StatusBadRequest, "invalid provider"),
			"Unsupported bank provider")
	}

	balances, err := h.bankService.GetBalances(c.Context(), &bank.GetBalancesRequest{
		UserID:       userID,
		BankProvider: provider,
		AccountID:    accountID,
	})

	if err != nil {
		return h.sendError(c, fiber.StatusInternalServerError, err, "Failed to fetch balances")
	}

	h.log.Info().
		Str("op", op).
		Int("user_id", userID).
		Str("bank_provider", providerStr).
		Str("account_id", accountID).
		Int("balances_count", len(balances)).
		Msg("balances fetched")

	return c.JSON(fiber.Map{
		"bank_provider": providerStr,
		"account_id":    accountID,
		"balances":      balances,
		"count":         len(balances),
	})
}

// GetTransactions godoc
// @Summary      Get account transactions
// @Description  Retrieve transaction history for a specific account from a connected bank
// @Tags         banks
// @Security     BearerAuth
// @Param        provider path string true "Bank provider" Enums(vbank, sbank, abank)
// @Param        accountId path string true "Account ID"
// @Produce      json
// @Success      200 {object} object{bank_provider=string,account_id=string,transactions=[]models.Transaction,count=int}
// @Failure      400 {object} ErrorResponse "Invalid parameters"
// @Failure      401 {object} ErrorResponse "Unauthorized"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /banks/{provider}/accounts/{accountId}/transactions [get]
func (h *BankHandler) GetTransactions(c *fiber.Ctx) error {
	const op = "handlers.BankHandler.GetTransactions"

	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	providerStr := c.Params("provider")
	if providerStr == "" {
		return h.sendError(c, fiber.StatusBadRequest,
			fiber.NewError(fiber.StatusBadRequest, "missing provider"),
			"bank provider is required")
	}

	accountID := c.Params("accountId")
	if accountID == "" {
		return h.sendError(c, fiber.StatusBadRequest,
			fiber.NewError(fiber.StatusBadRequest, "missing account ID"),
			"account ID is required")
	}

	provider := models.BankProvider(providerStr)
	if provider != models.VBankProvider && provider != models.SBankProvider && provider != models.ABankProvider {
		return h.sendError(c, fiber.StatusBadRequest,
			fiber.NewError(fiber.StatusBadRequest, "invalid provider"),
			"Unsupported bank provider")
	}

	transactions, err := h.bankService.GetTransactions(c.Context(), &bank.GetTransactionsRequest{
		UserID:       userID,
		BankProvider: provider,
		AccountID:    accountID,
	})

	if err != nil {
		return h.sendError(c, fiber.StatusInternalServerError, err, "Failed to fetch transactions")
	}

	h.log.Info().
		Str("op", op).
		Int("user_id", userID).
		Str("bank_provider", providerStr).
		Str("account_id", accountID).
		Int("transactions_count", len(transactions)).
		Msg("transactions fetched")

	return c.JSON(fiber.Map{
		"bank_provider": providerStr,
		"account_id":    accountID,
		"transactions":  transactions,
		"count":         len(transactions),
	})
}
