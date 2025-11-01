package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/kirill010106/multibank_service_app/backend/internal/models"
	"github.com/kirill010106/multibank_service_app/backend/internal/services/bank"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockBankService mocks Bank Service
type MockBankService struct {
	mock.Mock
}

func (m *MockBankService) ConnectBank(ctx context.Context, req *bank.ConnectBankRequest) (*bank.ConnectBankResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*bank.ConnectBankResponse), args.Error(1)
}

func (m *MockBankService) GetConnections(ctx context.Context, userID int) ([]*models.BankConnection, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.BankConnection), args.Error(1)
}

func (m *MockBankService) GetAccounts(ctx context.Context, req *bank.GetAccountsRequest) ([]*models.Account, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Account), args.Error(1)
}

func (m *MockBankService) DisconnectBank(ctx context.Context, userID int, provider models.BankProvider) error {
	args := m.Called(ctx, userID, provider)
	return args.Error(0)
}

func (m *MockBankService) GetDashboard(ctx context.Context, userID int) (*models.DashboardResponse, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DashboardResponse), args.Error(1)
}

// Helper to create test Fiber app with JWT middleware mock
func setupTestApp(handler *BankHandler) *fiber.App {
	app := fiber.New()

	// Mock JWT middleware - set user_id in Locals
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", 1) // Default test user ID
		return c.Next()
	})

	// Bank routes
	bankGroup := app.Group("/api/v1/banks")
	bankGroup.Post("/connect", handler.ConnectBank)
	bankGroup.Get("/", handler.GetConnections)
	bankGroup.Get("/:provider/accounts", handler.GetAccounts)
	bankGroup.Delete("/:provider", handler.DisconnectBank)

	return app
}

// Test ConnectBank Handler
func TestBankHandler_ConnectBank(t *testing.T) {
	log := zerolog.Nop()

	t.Run("successful connection", func(t *testing.T) {
		mockService := new(MockBankService)

		mockService.On("ConnectBank", mock.Anything, mock.MatchedBy(func(req *bank.ConnectBankRequest) bool {
			return req.UserID == 1 && req.BankProvider == models.VBankProvider && req.BankClientID == "team200-1"
		})).Return(&bank.ConnectBankResponse{
			Connection: &models.BankConnection{
				ID:           1,
				UserID:       1,
				BankProvider: models.VBankProvider,
				Status:       models.ConnectionActive,
			},
			ConsentID: "consent-123",
			Status:    "success",
			Message:   "Connected successfully to vbank",
		}, nil)

		handler := &BankHandler{
			bankService: mockService,
			log:         log,
		}

		app := setupTestApp(handler)

		reqBody := ConnectBankRequest{
			BankProvider: "vbank",
			BankClientID: "team200-1",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/v1/banks/connect", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, fiber.StatusCreated, resp.StatusCode)

		var result bank.ConnectBankResponse
		json.NewDecoder(resp.Body).Decode(&result)

		assert.Equal(t, "consent-123", result.ConsentID)
		assert.Equal(t, "success", result.Status)
		mockService.AssertExpectations(t)
	})

	t.Run("invalid request body", func(t *testing.T) {
		mockService := new(MockBankService)
		handler := &BankHandler{
			bankService: mockService,
			log:         log,
		}

		app := setupTestApp(handler)

		req := httptest.NewRequest("POST", "/api/v1/banks/connect", bytes.NewReader([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

		var result ErrorResponse
		json.NewDecoder(resp.Body).Decode(&result)
		assert.Contains(t, result.Message, "request body")
	})

	t.Run("validation failed - invalid provider", func(t *testing.T) {
		mockService := new(MockBankService)
		handler := &BankHandler{
			bankService: mockService,
			log:         log,
		}

		app := setupTestApp(handler)

		reqBody := ConnectBankRequest{
			BankProvider: "invalid_bank",
			BankClientID: "team200-1",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/v1/banks/connect", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

		var result ErrorResponse
		json.NewDecoder(resp.Body).Decode(&result)
		assert.Contains(t, result.Message, "validation")
	})

	t.Run("validation failed - empty bank_client_id", func(t *testing.T) {
		mockService := new(MockBankService)
		handler := &BankHandler{
			bankService: mockService,
			log:         log,
		}

		app := setupTestApp(handler)

		reqBody := ConnectBankRequest{
			BankProvider: "vbank",
			BankClientID: "",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/v1/banks/connect", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
	})

	t.Run("service error", func(t *testing.T) {
		mockService := new(MockBankService)

		mockService.On("ConnectBank", mock.Anything, mock.Anything).
			Return(nil, errors.New("service error"))

		handler := &BankHandler{
			bankService: mockService,
			log:         log,
		}

		app := setupTestApp(handler)

		reqBody := ConnectBankRequest{
			BankProvider: "vbank",
			BankClientID: "team200-1",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/v1/banks/connect", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)

		var result ErrorResponse
		json.NewDecoder(resp.Body).Decode(&result)
		assert.Contains(t, result.Message, "Failed to connect bank")
		mockService.AssertExpectations(t)
	})

	t.Run("missing user_id in context", func(t *testing.T) {
		mockService := new(MockBankService)
		handler := &BankHandler{
			bankService: mockService,
			log:         log,
		}

		// Create app WITHOUT JWT middleware mock
		app := fiber.New()
		app.Post("/api/v1/banks/connect", handler.ConnectBank)

		reqBody := ConnectBankRequest{
			BankProvider: "vbank",
			BankClientID: "team200-1",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/v1/banks/connect", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
	})
}

// Test GetConnections Handler
func TestBankHandler_GetConnections(t *testing.T) {
	log := zerolog.Nop()

	t.Run("successful fetch", func(t *testing.T) {
		mockService := new(MockBankService)

		connections := []*models.BankConnection{
			{
				ID:           1,
				UserID:       1,
				BankProvider: models.VBankProvider,
				Status:       models.ConnectionActive,
			},
			{
				ID:           2,
				UserID:       1,
				BankProvider: models.SBankProvider,
				Status:       models.ConnectionActive,
			},
		}

		mockService.On("GetConnections", mock.Anything, 1).Return(connections, nil)

		handler := &BankHandler{
			bankService: mockService,
			log:         log,
		}

		app := setupTestApp(handler)

		req := httptest.NewRequest("GET", "/api/v1/banks", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, fiber.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		assert.Equal(t, float64(2), result["count"])
		assert.NotNil(t, result["connections"])
		mockService.AssertExpectations(t)
	})

	t.Run("empty connections", func(t *testing.T) {
		mockService := new(MockBankService)

		mockService.On("GetConnections", mock.Anything, 1).Return([]*models.BankConnection{}, nil)

		handler := &BankHandler{
			bankService: mockService,
			log:         log,
		}

		app := setupTestApp(handler)

		req := httptest.NewRequest("GET", "/api/v1/banks", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, fiber.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		assert.Equal(t, float64(0), result["count"])
		mockService.AssertExpectations(t)
	})

	t.Run("service error", func(t *testing.T) {
		mockService := new(MockBankService)

		mockService.On("GetConnections", mock.Anything, 1).
			Return(nil, errors.New("database error"))

		handler := &BankHandler{
			bankService: mockService,
			log:         log,
		}

		app := setupTestApp(handler)

		req := httptest.NewRequest("GET", "/api/v1/banks", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)

		var result ErrorResponse
		json.NewDecoder(resp.Body).Decode(&result)
		assert.Contains(t, result.Message, "fetch")
		mockService.AssertExpectations(t)
	})
}

// Test GetAccounts Handler
func TestBankHandler_GetAccounts(t *testing.T) {
	log := zerolog.Nop()

	t.Run("successful fetch", func(t *testing.T) {
		mockService := new(MockBankService)

		accounts := []*models.Account{
			{AccountID: "acc-1", Currency: "RUB"},
			{AccountID: "acc-2", Currency: "USD"},
		}

		mockService.On("GetAccounts", mock.Anything, mock.MatchedBy(func(req *bank.GetAccountsRequest) bool {
			return req.UserID == 1 && req.BankProvider == models.VBankProvider
		})).Return(accounts, nil)

		handler := &BankHandler{
			bankService: mockService,
			log:         log,
		}

		app := setupTestApp(handler)

		req := httptest.NewRequest("GET", "/api/v1/banks/vbank/accounts", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, fiber.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		assert.Equal(t, "vbank", result["bank_provider"])
		assert.Equal(t, float64(2), result["count"])
		mockService.AssertExpectations(t)
	})

	t.Run("invalid provider", func(t *testing.T) {
		mockService := new(MockBankService)
		handler := &BankHandler{
			bankService: mockService,
			log:         log,
		}

		app := setupTestApp(handler)

		req := httptest.NewRequest("GET", "/api/v1/banks/invalid_bank/accounts", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

		var result ErrorResponse
		json.NewDecoder(resp.Body).Decode(&result)
		assert.Contains(t, result.Message, "bank provider")
	})

	t.Run("service error", func(t *testing.T) {
		mockService := new(MockBankService)

		mockService.On("GetAccounts", mock.Anything, mock.Anything).
			Return(nil, errors.New("bank API error"))

		handler := &BankHandler{
			bankService: mockService,
			log:         log,
		}

		app := setupTestApp(handler)

		req := httptest.NewRequest("GET", "/api/v1/banks/vbank/accounts", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)

		var result ErrorResponse
		json.NewDecoder(resp.Body).Decode(&result)
		assert.Contains(t, result.Message, "Failed to fetch accounts")
		mockService.AssertExpectations(t)
	})
}

// Test DisconnectBank Handler
func TestBankHandler_DisconnectBank(t *testing.T) {
	log := zerolog.Nop()

	t.Run("successful disconnect", func(t *testing.T) {
		mockService := new(MockBankService)

		mockService.On("DisconnectBank", mock.Anything, 1, models.VBankProvider).Return(nil)

		handler := &BankHandler{
			bankService: mockService,
			log:         log,
		}

		app := setupTestApp(handler)

		req := httptest.NewRequest("DELETE", "/api/v1/banks/vbank", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, fiber.StatusNoContent, resp.StatusCode)
		mockService.AssertExpectations(t)
	})

	t.Run("invalid provider", func(t *testing.T) {
		mockService := new(MockBankService)
		handler := &BankHandler{
			bankService: mockService,
			log:         log,
		}

		app := setupTestApp(handler)

		req := httptest.NewRequest("DELETE", "/api/v1/banks/invalid_bank", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)

		var result ErrorResponse
		json.NewDecoder(resp.Body).Decode(&result)
		assert.Contains(t, result.Message, "Unsupported bank provider")
	})

	t.Run("service error", func(t *testing.T) {
		mockService := new(MockBankService)

		mockService.On("DisconnectBank", mock.Anything, 1, models.VBankProvider).
			Return(errors.New("connection not found"))

		handler := &BankHandler{
			bankService: mockService,
			log:         log,
		}

		app := setupTestApp(handler)

		req := httptest.NewRequest("DELETE", "/api/v1/banks/vbank", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)

		var result ErrorResponse
		json.NewDecoder(resp.Body).Decode(&result)
		assert.Contains(t, result.Message, "Failed to disconnect bank")
		mockService.AssertExpectations(t)
	})
}
