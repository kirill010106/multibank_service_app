package bank

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/kirill010106/multibank_service_app/backend/internal/models"
	"github.com/kirill010106/multibank_service_app/backend/pkg/clients"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock Repository
type MockBankConnectionRepository struct {
	mock.Mock
}

func (m *MockBankConnectionRepository) Create(ctx context.Context, conn *models.BankConnection) error {
	args := m.Called(ctx, conn)
	if args.Get(0) == nil {
		// Simulate DB auto-generated fields
		conn.ID = 1
		conn.CreatedAt = time.Now()
		conn.UpdatedAt = time.Now()
		return nil
	}
	return args.Error(0)
}

func (m *MockBankConnectionRepository) FindByUserAndBank(ctx context.Context, userID int, provider models.BankProvider) (*models.BankConnection, error) {
	args := m.Called(ctx, userID, provider)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.BankConnection), args.Error(1)
}

func (m *MockBankConnectionRepository) FindAllByUser(ctx context.Context, userID int) ([]*models.BankConnection, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.BankConnection), args.Error(1)
}

func (m *MockBankConnectionRepository) UpdateToken(ctx context.Context, id int, token string, expiresAt time.Time) error {
	args := m.Called(ctx, id, token, expiresAt)
	return args.Error(0)
}

func (m *MockBankConnectionRepository) UpdateConsent(ctx context.Context, id int, consentID string, status models.ConnectionStatus) error {
	args := m.Called(ctx, id, consentID, status)
	return args.Error(0)
}

func (m *MockBankConnectionRepository) UpdateStatus(ctx context.Context, id int, status models.ConnectionStatus) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *MockBankConnectionRepository) Delete(ctx context.Context, id int) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// Mock Bank Client
type MockBankClient struct {
	mock.Mock
}

func (m *MockBankClient) GetBankToken(ctx context.Context) (string, int64, error) {
	args := m.Called(ctx)
	return args.String(0), args.Get(1).(int64), args.Error(2)
}

func (m *MockBankClient) CreateConsent(ctx context.Context, bankToken, bankClientID string) (string, error) {
	args := m.Called(ctx, bankToken, bankClientID)
	return args.String(0), args.Error(1)
}

func (m *MockBankClient) GetAccounts(ctx context.Context, bankToken, bankClientID, consentID string) ([]*models.Account, error) {
	args := m.Called(ctx, bankToken, bankClientID, consentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Account), args.Error(1)
}

func (m *MockBankClient) GetBalances(ctx context.Context, bankToken, bankClientID, consentID, accountID string) ([]*models.Balance, error) {
	args := m.Called(ctx, bankToken, bankClientID, consentID, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Balance), args.Error(1)
}

func (m *MockBankClient) GetTransactions(ctx context.Context, bankToken, bankClientID, consentID, accountID string) ([]*models.Transaction, error) {
	args := m.Called(ctx, bankToken, bankClientID, consentID, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Transaction), args.Error(1)
}

func (m *MockBankClient) GetBankProvider() models.BankProvider {
	args := m.Called()
	return args.Get(0).(models.BankProvider)
}

// Tests
func TestService_ConnectBank(t *testing.T) {
	log := zerolog.Nop()
	ctx := context.Background()

	tests := []struct {
		name          string
		request       *ConnectBankRequest
		mockSetup     func(*MockBankConnectionRepository, *MockBankClient)
		expectedError bool
		validate      func(*testing.T, *ConnectBankResponse)
	}{
		{
			name: "successful connection",
			request: &ConnectBankRequest{
				UserID:       1,
				BankProvider: models.VBankProvider,
				BankClientID: "team200-1",
			},
			mockSetup: func(repo *MockBankConnectionRepository, client *MockBankClient) {
				// Mock GetBankToken
				client.On("GetBankToken", ctx).Return("mock_token", int64(86400), nil)

				// Mock CreateConsent
				client.On("CreateConsent", ctx, "mock_token", "team200-1").Return("consent-123", nil)

				// Mock Create
				repo.On("Create", ctx, mock.AnythingOfType("*models.BankConnection")).Return(nil)
			},
			expectedError: false,
			validate: func(t *testing.T, resp *ConnectBankResponse) {
				assert.Equal(t, "consent-123", resp.ConsentID)
				assert.Equal(t, "success", resp.Status)
				assert.NotNil(t, resp.Connection)
			},
		},
		{
			name: "bank token error",
			request: &ConnectBankRequest{
				UserID:       1,
				BankProvider: models.VBankProvider,
				BankClientID: "team200-1",
			},
			mockSetup: func(repo *MockBankConnectionRepository, client *MockBankClient) {
				client.On("GetBankToken", ctx).Return("", int64(0), errors.New("token error"))
			},
			expectedError: true,
		},
		{
			name: "consent creation error",
			request: &ConnectBankRequest{
				UserID:       1,
				BankProvider: models.VBankProvider,
				BankClientID: "team200-1",
			},
			mockSetup: func(repo *MockBankConnectionRepository, client *MockBankClient) {
				client.On("GetBankToken", ctx).Return("mock_token", int64(86400), nil)
				client.On("CreateConsent", ctx, "mock_token", "team200-1").Return("", errors.New("consent error"))
			},
			expectedError: true,
		},
		{
			name: "unsupported bank provider",
			request: &ConnectBankRequest{
				UserID:       1,
				BankProvider: models.SBankProvider,
				BankClientID: "sbank-client",
			},
			mockSetup: func(repo *MockBankConnectionRepository, client *MockBankClient) {
				// No mocks needed - should fail before calling any methods
			},
			expectedError: true,
		},
		{
			name: "database save error",
			request: &ConnectBankRequest{
				UserID:       1,
				BankProvider: models.VBankProvider,
				BankClientID: "team200-1",
			},
			mockSetup: func(repo *MockBankConnectionRepository, client *MockBankClient) {
				client.On("GetBankToken", ctx).Return("mock_token", int64(86400), nil)
				client.On("CreateConsent", ctx, "mock_token", "team200-1").Return("consent-123", nil)
				repo.On("Create", ctx, mock.AnythingOfType("*models.BankConnection")).
					Return(errors.New("database error"))
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(MockBankConnectionRepository)
			mockClient := new(MockBankClient)

			tt.mockSetup(mockRepo, mockClient)

			bankClients := map[models.BankProvider]clients.BankClient{
				models.VBankProvider: mockClient,
			}

			service := NewService(mockRepo, bankClients, log)

			resp, err := service.ConnectBank(ctx, tt.request)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				if tt.validate != nil {
					tt.validate(t, resp)
				}
			}

			mockRepo.AssertExpectations(t)
			mockClient.AssertExpectations(t)
		})
	}
}

func TestService_GetAccounts(t *testing.T) {
	log := zerolog.Nop()
	ctx := context.Background()

	futureTime := time.Now().Add(1 * time.Hour)
	token := "valid_token"
	consentID := "consent-123"

	tests := []struct {
		name          string
		request       *GetAccountsRequest
		mockSetup     func(*MockBankConnectionRepository, *MockBankClient)
		expectedError bool
		validate      func(*testing.T, []*models.Account)
	}{
		{
			name: "successful fetch",
			request: &GetAccountsRequest{
				UserID:       1,
				BankProvider: models.VBankProvider,
			},
			mockSetup: func(repo *MockBankConnectionRepository, client *MockBankClient) {
				connection := &models.BankConnection{
					ID:             1,
					UserID:         1,
					BankProvider:   models.VBankProvider,
					BankClientID:   "team200-1",
					ConsentID:      &consentID,
					AccessToken:    &token,
					TokenExpiresAt: &futureTime,
					Status:         models.ConnectionActive,
				}

				repo.On("FindByUserAndBank", ctx, 1, models.VBankProvider).Return(connection, nil)

				accounts := []*models.Account{
					{AccountID: "acc-1", Currency: "RUB"},
				}
				client.On("GetAccounts", ctx, token, "team200-1", consentID).Return(accounts, nil)
			},
			expectedError: false,
			validate: func(t *testing.T, accounts []*models.Account) {
				assert.Len(t, accounts, 1)
				assert.Equal(t, "acc-1", accounts[0].AccountID)
			},
		},
		{
			name: "connection not found",
			request: &GetAccountsRequest{
				UserID:       1,
				BankProvider: models.VBankProvider,
			},
			mockSetup: func(repo *MockBankConnectionRepository, client *MockBankClient) {
				repo.On("FindByUserAndBank", ctx, 1, models.VBankProvider).Return(nil, errors.New("not found"))
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(MockBankConnectionRepository)
			mockClient := new(MockBankClient)

			tt.mockSetup(mockRepo, mockClient)

			bankClients := map[models.BankProvider]clients.BankClient{
				models.VBankProvider: mockClient,
			}

			service := NewService(mockRepo, bankClients, log)

			accounts, err := service.GetAccounts(ctx, tt.request)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, accounts)
				}
			}

			mockRepo.AssertExpectations(t)
			mockClient.AssertExpectations(t)
		})
	}
}

// Additional tests for GetAccounts edge cases
func TestService_GetAccounts_AdvancedCases(t *testing.T) {
	log := zerolog.Nop()
	ctx := context.Background()

	t.Run("connection not active", func(t *testing.T) {
		mockRepo := new(MockBankConnectionRepository)
		mockClient := new(MockBankClient)

		connection := &models.BankConnection{
			ID:           1,
			UserID:       1,
			BankProvider: models.VBankProvider,
			Status:       models.ConnectionExpired,
		}

		mockRepo.On("FindByUserAndBank", ctx, 1, models.VBankProvider).Return(connection, nil)

		service := NewService(mockRepo, map[models.BankProvider]clients.BankClient{
			models.VBankProvider: mockClient,
		}, log)

		_, err := service.GetAccounts(ctx, &GetAccountsRequest{
			UserID:       1,
			BankProvider: models.VBankProvider,
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connection is not active")
	})

	t.Run("token expired - refresh success", func(t *testing.T) {
		mockRepo := new(MockBankConnectionRepository)
		mockClient := new(MockBankClient)

		oldToken := "old_token"
		consentID := "consent-123"
		expiredTime := time.Now().Add(-1 * time.Hour)

		connection := &models.BankConnection{
			ID:             1,
			UserID:         1,
			BankProvider:   models.VBankProvider,
			BankClientID:   "team200-1",
			ConsentID:      &consentID,
			AccessToken:    &oldToken,
			TokenExpiresAt: &expiredTime,
			Status:         models.ConnectionActive,
		}

		mockRepo.On("FindByUserAndBank", ctx, 1, models.VBankProvider).Return(connection, nil)

		// Mock token refresh
		newToken := "new_token"
		mockClient.On("GetBankToken", ctx).Return(newToken, int64(86400), nil)
		mockRepo.On("UpdateToken", ctx, 1, newToken, mock.AnythingOfType("time.Time")).Return(nil)

		// Mock GetAccounts with new token
		accounts := []*models.Account{{AccountID: "acc-1", Currency: "RUB"}}
		mockClient.On("GetAccounts", ctx, newToken, "team200-1", consentID).Return(accounts, nil)

		service := NewService(mockRepo, map[models.BankProvider]clients.BankClient{
			models.VBankProvider: mockClient,
		}, log)

		result, err := service.GetAccounts(ctx, &GetAccountsRequest{
			UserID:       1,
			BankProvider: models.VBankProvider,
		})

		assert.NoError(t, err)
		assert.Len(t, result, 1)
		mockRepo.AssertExpectations(t)
		mockClient.AssertExpectations(t)
	})

	t.Run("token expired - refresh failed", func(t *testing.T) {
		mockRepo := new(MockBankConnectionRepository)
		mockClient := new(MockBankClient)

		oldToken := "old_token"
		consentID := "consent-123"
		expiredTime := time.Now().Add(-1 * time.Hour)

		connection := &models.BankConnection{
			ID:             1,
			UserID:         1,
			BankProvider:   models.VBankProvider,
			BankClientID:   "team200-1",
			ConsentID:      &consentID,
			AccessToken:    &oldToken,
			TokenExpiresAt: &expiredTime,
			Status:         models.ConnectionActive,
		}

		mockRepo.On("FindByUserAndBank", ctx, 1, models.VBankProvider).Return(connection, nil)
		mockClient.On("GetBankToken", ctx).Return("", int64(0), errors.New("refresh failed"))

		service := NewService(mockRepo, map[models.BankProvider]clients.BankClient{
			models.VBankProvider: mockClient,
		}, log)

		_, err := service.GetAccounts(ctx, &GetAccountsRequest{
			UserID:       1,
			BankProvider: models.VBankProvider,
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to refresh access token")
	})

	t.Run("unsupported bank provider", func(t *testing.T) {
		mockRepo := new(MockBankConnectionRepository)

		futureTime := time.Now().Add(1 * time.Hour)
		token := "valid_token"
		consentID := "consent-123"

		connection := &models.BankConnection{
			ID:             1,
			UserID:         1,
			BankProvider:   models.SBankProvider,
			BankClientID:   "sbank-client",
			ConsentID:      &consentID,
			AccessToken:    &token,
			TokenExpiresAt: &futureTime,
			Status:         models.ConnectionActive,
		}

		mockRepo.On("FindByUserAndBank", ctx, 1, models.SBankProvider).Return(connection, nil)

		service := NewService(mockRepo, map[models.BankProvider]clients.BankClient{
			models.VBankProvider: new(MockBankClient),
		}, log)

		_, err := service.GetAccounts(ctx, &GetAccountsRequest{
			UserID:       1,
			BankProvider: models.SBankProvider,
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported bank provider")
	})

	t.Run("missing access token", func(t *testing.T) {
		mockRepo := new(MockBankConnectionRepository)
		mockClient := new(MockBankClient)

		futureTime := time.Now().Add(1 * time.Hour)
		consentID := "consent-123"

		connection := &models.BankConnection{
			ID:             1,
			UserID:         1,
			BankProvider:   models.VBankProvider,
			BankClientID:   "team200-1",
			ConsentID:      &consentID,
			AccessToken:    nil,
			TokenExpiresAt: &futureTime,
			Status:         models.ConnectionActive,
		}

		mockRepo.On("FindByUserAndBank", ctx, 1, models.VBankProvider).Return(connection, nil)

		service := NewService(mockRepo, map[models.BankProvider]clients.BankClient{
			models.VBankProvider: mockClient,
		}, log)

		_, err := service.GetAccounts(ctx, &GetAccountsRequest{
			UserID:       1,
			BankProvider: models.VBankProvider,
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing access token")
	})

	t.Run("missing consent ID", func(t *testing.T) {
		mockRepo := new(MockBankConnectionRepository)
		mockClient := new(MockBankClient)

		futureTime := time.Now().Add(1 * time.Hour)
		token := "valid_token"

		connection := &models.BankConnection{
			ID:             1,
			UserID:         1,
			BankProvider:   models.VBankProvider,
			BankClientID:   "team200-1",
			ConsentID:      nil,
			AccessToken:    &token,
			TokenExpiresAt: &futureTime,
			Status:         models.ConnectionActive,
		}

		mockRepo.On("FindByUserAndBank", ctx, 1, models.VBankProvider).Return(connection, nil)

		service := NewService(mockRepo, map[models.BankProvider]clients.BankClient{
			models.VBankProvider: mockClient,
		}, log)

		_, err := service.GetAccounts(ctx, &GetAccountsRequest{
			UserID:       1,
			BankProvider: models.VBankProvider,
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing consent ID")
	})

	t.Run("bank API error", func(t *testing.T) {
		mockRepo := new(MockBankConnectionRepository)
		mockClient := new(MockBankClient)

		futureTime := time.Now().Add(1 * time.Hour)
		token := "valid_token"
		consentID := "consent-123"

		connection := &models.BankConnection{
			ID:             1,
			UserID:         1,
			BankProvider:   models.VBankProvider,
			BankClientID:   "team200-1",
			ConsentID:      &consentID,
			AccessToken:    &token,
			TokenExpiresAt: &futureTime,
			Status:         models.ConnectionActive,
		}

		mockRepo.On("FindByUserAndBank", ctx, 1, models.VBankProvider).Return(connection, nil)
		mockClient.On("GetAccounts", ctx, token, "team200-1", consentID).Return(nil, errors.New("API error"))

		service := NewService(mockRepo, map[models.BankProvider]clients.BankClient{
			models.VBankProvider: mockClient,
		}, log)

		_, err := service.GetAccounts(ctx, &GetAccountsRequest{
			UserID:       1,
			BankProvider: models.VBankProvider,
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch accounts")
	})
}

// Test GetConnections
func TestService_GetConnections(t *testing.T) {
	log := zerolog.Nop()
	ctx := context.Background()

	t.Run("successful fetch with multiple connections", func(t *testing.T) {
		mockRepo := new(MockBankConnectionRepository)

		token := "token"
		consentID := "consent-123"
		expiresAt := time.Now().Add(1 * time.Hour)

		connections := []*models.BankConnection{
			{
				ID:             1,
				UserID:         1,
				BankProvider:   models.VBankProvider,
				BankClientID:   "vbank-client",
				ConsentID:      &consentID,
				AccessToken:    &token,
				TokenExpiresAt: &expiresAt,
				Status:         models.ConnectionActive,
			},
			{
				ID:             2,
				UserID:         1,
				BankProvider:   models.SBankProvider,
				BankClientID:   "sbank-client",
				ConsentID:      &consentID,
				AccessToken:    &token,
				TokenExpiresAt: &expiresAt,
				Status:         models.ConnectionActive,
			},
		}

		mockRepo.On("FindAllByUser", ctx, 1).Return(connections, nil)

		service := NewService(mockRepo, map[models.BankProvider]clients.BankClient{}, log)

		result, err := service.GetConnections(ctx, 1)

		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, models.VBankProvider, result[0].BankProvider)
		assert.Equal(t, models.SBankProvider, result[1].BankProvider)
		mockRepo.AssertExpectations(t)
	})

	t.Run("empty connections list", func(t *testing.T) {
		mockRepo := new(MockBankConnectionRepository)

		mockRepo.On("FindAllByUser", ctx, 1).Return([]*models.BankConnection{}, nil)

		service := NewService(mockRepo, map[models.BankProvider]clients.BankClient{}, log)

		result, err := service.GetConnections(ctx, 1)

		assert.NoError(t, err)
		assert.Empty(t, result)
		mockRepo.AssertExpectations(t)
	})

	t.Run("database error", func(t *testing.T) {
		mockRepo := new(MockBankConnectionRepository)

		mockRepo.On("FindAllByUser", ctx, 1).Return(nil, errors.New("database error"))

		service := NewService(mockRepo, map[models.BankProvider]clients.BankClient{}, log)

		result, err := service.GetConnections(ctx, 1)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "database error")
		mockRepo.AssertExpectations(t)
	})
}

// Test refreshToken
func TestService_refreshToken(t *testing.T) {
	log := zerolog.Nop()
	ctx := context.Background()

	t.Run("successful refresh", func(t *testing.T) {
		mockRepo := new(MockBankConnectionRepository)
		mockClient := new(MockBankClient)

		oldToken := "old_token"
		consentID := "consent-123"
		expiredTime := time.Now().Add(-1 * time.Hour)

		connection := &models.BankConnection{
			ID:             1,
			UserID:         1,
			BankProvider:   models.VBankProvider,
			BankClientID:   "team200-1",
			ConsentID:      &consentID,
			AccessToken:    &oldToken,
			TokenExpiresAt: &expiredTime,
			Status:         models.ConnectionActive,
		}

		newToken := "new_token"
		mockClient.On("GetBankToken", ctx).Return(newToken, int64(86400), nil)
		mockRepo.On("UpdateToken", ctx, 1, newToken, mock.AnythingOfType("time.Time")).Return(nil)

		service := NewService(mockRepo, map[models.BankProvider]clients.BankClient{
			models.VBankProvider: mockClient,
		}, log)

		err := service.refreshToken(ctx, connection)

		assert.NoError(t, err)
		assert.Equal(t, newToken, *connection.AccessToken)
		assert.True(t, time.Now().Before(*connection.TokenExpiresAt))
		mockRepo.AssertExpectations(t)
		mockClient.AssertExpectations(t)
	})

	t.Run("unsupported bank provider", func(t *testing.T) {
		mockRepo := new(MockBankConnectionRepository)

		connection := &models.BankConnection{
			ID:           1,
			BankProvider: models.SBankProvider,
		}

		service := NewService(mockRepo, map[models.BankProvider]clients.BankClient{
			models.VBankProvider: new(MockBankClient),
		}, log)

		err := service.refreshToken(ctx, connection)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported bank provider")
	})

	t.Run("GetBankToken error", func(t *testing.T) {
		mockRepo := new(MockBankConnectionRepository)
		mockClient := new(MockBankClient)

		connection := &models.BankConnection{
			ID:           1,
			BankProvider: models.VBankProvider,
		}

		mockClient.On("GetBankToken", ctx).Return("", int64(0), errors.New("token error"))

		service := NewService(mockRepo, map[models.BankProvider]clients.BankClient{
			models.VBankProvider: mockClient,
		}, log)

		err := service.refreshToken(ctx, connection)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get bank token")
		mockClient.AssertExpectations(t)
	})

	t.Run("UpdateToken database error", func(t *testing.T) {
		mockRepo := new(MockBankConnectionRepository)
		mockClient := new(MockBankClient)

		connection := &models.BankConnection{
			ID:           1,
			BankProvider: models.VBankProvider,
		}

		newToken := "new_token"
		mockClient.On("GetBankToken", ctx).Return(newToken, int64(86400), nil)
		mockRepo.On("UpdateToken", ctx, 1, newToken, mock.AnythingOfType("time.Time")).
			Return(errors.New("database error"))

		service := NewService(mockRepo, map[models.BankProvider]clients.BankClient{
			models.VBankProvider: mockClient,
		}, log)

		err := service.refreshToken(ctx, connection)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update token in database")
		mockRepo.AssertExpectations(t)
		mockClient.AssertExpectations(t)
	})
}

// Test DisconnectBank
func TestService_DisconnectBank(t *testing.T) {
	log := zerolog.Nop()
	ctx := context.Background()

	t.Run("successful disconnect", func(t *testing.T) {
		mockRepo := new(MockBankConnectionRepository)

		connection := &models.BankConnection{
			ID:           1,
			UserID:       1,
			BankProvider: models.VBankProvider,
			Status:       models.ConnectionActive,
		}

		mockRepo.On("FindByUserAndBank", ctx, 1, models.VBankProvider).Return(connection, nil)
		mockRepo.On("Delete", ctx, 1).Return(nil)

		service := NewService(mockRepo, map[models.BankProvider]clients.BankClient{}, log)

		err := service.DisconnectBank(ctx, 1, models.VBankProvider)

		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("connection not found", func(t *testing.T) {
		mockRepo := new(MockBankConnectionRepository)

		mockRepo.On("FindByUserAndBank", ctx, 1, models.VBankProvider).
			Return(nil, errors.New("not found"))

		service := NewService(mockRepo, map[models.BankProvider]clients.BankClient{}, log)

		err := service.DisconnectBank(ctx, 1, models.VBankProvider)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connection not found")
		mockRepo.AssertExpectations(t)
	})

	t.Run("delete error", func(t *testing.T) {
		mockRepo := new(MockBankConnectionRepository)

		connection := &models.BankConnection{
			ID:           1,
			UserID:       1,
			BankProvider: models.VBankProvider,
			Status:       models.ConnectionActive,
		}

		mockRepo.On("FindByUserAndBank", ctx, 1, models.VBankProvider).Return(connection, nil)
		mockRepo.On("Delete", ctx, 1).Return(errors.New("delete failed"))

		service := NewService(mockRepo, map[models.BankProvider]clients.BankClient{}, log)

		err := service.DisconnectBank(ctx, 1, models.VBankProvider)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "delete connection")
		mockRepo.AssertExpectations(t)
	})
}

// Test GetDashboard
func TestService_GetDashboard(t *testing.T) {
	log := zerolog.Nop()
	ctx := context.Background()

	t.Run("successful dashboard with multiple banks", func(t *testing.T) {
		mockRepo := new(MockBankConnectionRepository)
		mockVBankClient := new(MockBankClient)

		token := "token"
		consentID := "consent-123"
		expiresAt := time.Now().Add(1 * time.Hour)

		connections := []*models.BankConnection{
			{
				ID:             1,
				UserID:         1,
				BankProvider:   models.VBankProvider,
				BankClientID:   "vbank-client",
				ConsentID:      &consentID,
				AccessToken:    &token,
				TokenExpiresAt: &expiresAt,
				Status:         models.ConnectionActive,
			},
		}

		mockRepo.On("FindAllByUser", ctx, 1).Return(connections, nil)

		accounts := []*models.Account{
			{AccountID: "acc-1", Currency: "RUB"},
			{AccountID: "acc-2", Currency: "USD"},
		}
		mockVBankClient.On("GetAccounts", ctx, token, "vbank-client", consentID).Return(accounts, nil)

		service := NewService(mockRepo, map[models.BankProvider]clients.BankClient{
			models.VBankProvider: mockVBankClient,
		}, log)

		result, err := service.GetDashboard(ctx, 1)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 1, result.UserID)
		assert.Equal(t, 1, result.TotalBanks)
		assert.Equal(t, 1, result.ActiveBanks)
		assert.Equal(t, 2, result.TotalAccounts)
		assert.Len(t, result.Banks, 1)
		assert.Equal(t, models.VBankProvider, result.Banks[0].Provider)
		assert.Equal(t, 2, result.Banks[0].AccountsCount)
		assert.Empty(t, result.Banks[0].Error)
		mockRepo.AssertExpectations(t)
		mockVBankClient.AssertExpectations(t)
	})

	t.Run("empty connections list", func(t *testing.T) {
		mockRepo := new(MockBankConnectionRepository)

		mockRepo.On("FindAllByUser", ctx, 1).Return([]*models.BankConnection{}, nil)

		service := NewService(mockRepo, map[models.BankProvider]clients.BankClient{}, log)

		result, err := service.GetDashboard(ctx, 1)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 0, result.TotalBanks)
		assert.Equal(t, 0, result.ActiveBanks)
		assert.Equal(t, 0, result.TotalAccounts)
		assert.Empty(t, result.Banks)
		mockRepo.AssertExpectations(t)
	})

	t.Run("database error fetching connections", func(t *testing.T) {
		mockRepo := new(MockBankConnectionRepository)

		mockRepo.On("FindAllByUser", ctx, 1).Return(nil, errors.New("database error"))

		service := NewService(mockRepo, map[models.BankProvider]clients.BankClient{}, log)

		result, err := service.GetDashboard(ctx, 1)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "database error")
		mockRepo.AssertExpectations(t)
	})

	t.Run("inactive connection skipped", func(t *testing.T) {
		mockRepo := new(MockBankConnectionRepository)

		connections := []*models.BankConnection{
			{
				ID:           1,
				UserID:       1,
				BankProvider: models.VBankProvider,
				Status:       models.ConnectionExpired,
			},
		}

		mockRepo.On("FindAllByUser", ctx, 1).Return(connections, nil)

		service := NewService(mockRepo, map[models.BankProvider]clients.BankClient{
			models.VBankProvider: new(MockBankClient),
		}, log)

		result, err := service.GetDashboard(ctx, 1)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 1, result.TotalBanks)
		assert.Equal(t, 0, result.ActiveBanks) // Not active
		assert.Equal(t, 0, result.TotalAccounts)
		assert.Len(t, result.Banks, 1)
		assert.Equal(t, "connection not active", result.Banks[0].Error)
		mockRepo.AssertExpectations(t)
	})

	t.Run("token expired - refresh success - fetch accounts", func(t *testing.T) {
		mockRepo := new(MockBankConnectionRepository)
		mockVBankClient := new(MockBankClient)

		oldToken := "old-token"
		consentID := "consent-123"
		expiredTime := time.Now().Add(-1 * time.Hour)

		connections := []*models.BankConnection{
			{
				ID:             1,
				UserID:         1,
				BankProvider:   models.VBankProvider,
				BankClientID:   "vbank-client",
				ConsentID:      &consentID,
				AccessToken:    &oldToken,
				TokenExpiresAt: &expiredTime,
				Status:         models.ConnectionActive,
			},
		}

		mockRepo.On("FindAllByUser", ctx, 1).Return(connections, nil)

		// Token refresh
		newToken := "new-token"
		mockVBankClient.On("GetBankToken", ctx).Return(newToken, int64(86400), nil)
		mockRepo.On("UpdateToken", ctx, 1, newToken, mock.AnythingOfType("time.Time")).Return(nil)

		// GetAccounts with new token
		accounts := []*models.Account{{AccountID: "acc-1", Currency: "RUB"}}
		mockVBankClient.On("GetAccounts", ctx, newToken, "vbank-client", consentID).Return(accounts, nil)

		service := NewService(mockRepo, map[models.BankProvider]clients.BankClient{
			models.VBankProvider: mockVBankClient,
		}, log)

		result, err := service.GetDashboard(ctx, 1)

		assert.NoError(t, err)
		assert.Equal(t, 1, result.ActiveBanks)
		assert.Equal(t, 1, result.TotalAccounts)
		assert.Empty(t, result.Banks[0].Error)
		mockRepo.AssertExpectations(t)
		mockVBankClient.AssertExpectations(t)
	})

	t.Run("token expired - refresh failed", func(t *testing.T) {
		mockRepo := new(MockBankConnectionRepository)
		mockVBankClient := new(MockBankClient)

		oldToken := "old-token"
		consentID := "consent-123"
		expiredTime := time.Now().Add(-1 * time.Hour)

		connections := []*models.BankConnection{
			{
				ID:             1,
				UserID:         1,
				BankProvider:   models.VBankProvider,
				BankClientID:   "vbank-client",
				ConsentID:      &consentID,
				AccessToken:    &oldToken,
				TokenExpiresAt: &expiredTime,
				Status:         models.ConnectionActive,
			},
		}

		mockRepo.On("FindAllByUser", ctx, 1).Return(connections, nil)
		mockVBankClient.On("GetBankToken", ctx).Return("", int64(0), errors.New("refresh failed"))

		service := NewService(mockRepo, map[models.BankProvider]clients.BankClient{
			models.VBankProvider: mockVBankClient,
		}, log)

		result, err := service.GetDashboard(ctx, 1)

		assert.NoError(t, err) // Dashboard doesn't fail, just marks bank as error
		assert.Equal(t, 1, result.TotalBanks)
		assert.Equal(t, 0, result.ActiveBanks)
		assert.Contains(t, result.Banks[0].Error, "token refresh failed")
		mockRepo.AssertExpectations(t)
		mockVBankClient.AssertExpectations(t)
	})

	t.Run("unsupported bank provider", func(t *testing.T) {
		mockRepo := new(MockBankConnectionRepository)

		token := "token"
		consentID := "consent-123"
		expiresAt := time.Now().Add(1 * time.Hour)

		connections := []*models.BankConnection{
			{
				ID:             1,
				UserID:         1,
				BankProvider:   models.SBankProvider,
				BankClientID:   "sbank-client",
				ConsentID:      &consentID,
				AccessToken:    &token,
				TokenExpiresAt: &expiresAt,
				Status:         models.ConnectionActive,
			},
		}

		mockRepo.On("FindAllByUser", ctx, 1).Return(connections, nil)

		service := NewService(mockRepo, map[models.BankProvider]clients.BankClient{
			models.VBankProvider: new(MockBankClient), // Only VBank, no SBank
		}, log)

		result, err := service.GetDashboard(ctx, 1)

		assert.NoError(t, err)
		assert.Equal(t, 1, result.TotalBanks)
		assert.Equal(t, 0, result.ActiveBanks)
		assert.Equal(t, "unsupported bank provider", result.Banks[0].Error)
		mockRepo.AssertExpectations(t)
	})

	t.Run("missing credentials", func(t *testing.T) {
		mockRepo := new(MockBankConnectionRepository)

		expiresAt := time.Now().Add(1 * time.Hour)

		connections := []*models.BankConnection{
			{
				ID:             1,
				UserID:         1,
				BankProvider:   models.VBankProvider,
				BankClientID:   "vbank-client",
				ConsentID:      nil, // Missing
				AccessToken:    nil, // Missing
				TokenExpiresAt: &expiresAt,
				Status:         models.ConnectionActive,
			},
		}

		mockRepo.On("FindAllByUser", ctx, 1).Return(connections, nil)

		service := NewService(mockRepo, map[models.BankProvider]clients.BankClient{
			models.VBankProvider: new(MockBankClient),
		}, log)

		result, err := service.GetDashboard(ctx, 1)

		assert.NoError(t, err)
		assert.Equal(t, 1, result.TotalBanks)
		assert.Equal(t, 0, result.ActiveBanks)
		assert.Equal(t, "missing credentials", result.Banks[0].Error)
		mockRepo.AssertExpectations(t)
	})

	t.Run("bank API error", func(t *testing.T) {
		mockRepo := new(MockBankConnectionRepository)
		mockVBankClient := new(MockBankClient)

		token := "token"
		consentID := "consent-123"
		expiresAt := time.Now().Add(1 * time.Hour)

		connections := []*models.BankConnection{
			{
				ID:             1,
				UserID:         1,
				BankProvider:   models.VBankProvider,
				BankClientID:   "vbank-client",
				ConsentID:      &consentID,
				AccessToken:    &token,
				TokenExpiresAt: &expiresAt,
				Status:         models.ConnectionActive,
			},
		}

		mockRepo.On("FindAllByUser", ctx, 1).Return(connections, nil)
		mockVBankClient.On("GetAccounts", ctx, token, "vbank-client", consentID).
			Return(nil, errors.New("API timeout"))

		service := NewService(mockRepo, map[models.BankProvider]clients.BankClient{
			models.VBankProvider: mockVBankClient,
		}, log)

		result, err := service.GetDashboard(ctx, 1)

		assert.NoError(t, err)
		assert.Equal(t, 1, result.TotalBanks)
		assert.Equal(t, 0, result.ActiveBanks)
		assert.Contains(t, result.Banks[0].Error, "bank api error")
		mockRepo.AssertExpectations(t)
		mockVBankClient.AssertExpectations(t)
	})

	t.Run("mixed banks - one success one error", func(t *testing.T) {
		mockRepo := new(MockBankConnectionRepository)
		mockVBankClient := new(MockBankClient)

		token := "token"
		consentID := "consent-123"
		expiresAt := time.Now().Add(1 * time.Hour)

		connections := []*models.BankConnection{
			{
				ID:             1,
				UserID:         1,
				BankProvider:   models.VBankProvider,
				BankClientID:   "vbank-client",
				ConsentID:      &consentID,
				AccessToken:    &token,
				TokenExpiresAt: &expiresAt,
				Status:         models.ConnectionActive,
			},
			{
				ID:           2,
				UserID:       1,
				BankProvider: models.SBankProvider,
				Status:       models.ConnectionExpired, // Inactive
			},
		}

		mockRepo.On("FindAllByUser", ctx, 1).Return(connections, nil)

		accounts := []*models.Account{{AccountID: "acc-1", Currency: "RUB"}}
		mockVBankClient.On("GetAccounts", ctx, token, "vbank-client", consentID).Return(accounts, nil)

		service := NewService(mockRepo, map[models.BankProvider]clients.BankClient{
			models.VBankProvider: mockVBankClient,
		}, log)

		result, err := service.GetDashboard(ctx, 1)

		assert.NoError(t, err)
		assert.Equal(t, 2, result.TotalBanks)
		assert.Equal(t, 1, result.ActiveBanks) // Only VBank active
		assert.Equal(t, 1, result.TotalAccounts)
		assert.Len(t, result.Banks, 2)

		// VBank success
		assert.Empty(t, result.Banks[0].Error)
		assert.Equal(t, 1, result.Banks[0].AccountsCount)

		// SBank error
		assert.Equal(t, "connection not active", result.Banks[1].Error)

		mockRepo.AssertExpectations(t)
		mockVBankClient.AssertExpectations(t)
	})
}

// Test GetBalances
func TestService_GetBalances(t *testing.T) {
	log := zerolog.Nop()
	ctx := context.Background()

	tests := []struct {
		name          string
		request       *GetBalancesRequest
		mockSetup     func(*MockBankConnectionRepository, *MockBankClient)
		expectedError bool
		validate      func(*testing.T, []*models.Balance)
	}{
		{
			name: "successful fetch",
			request: &GetBalancesRequest{
				UserID:       1,
				BankProvider: models.VBankProvider,
				AccountID:    "acc-123",
			},
			mockSetup: func(repo *MockBankConnectionRepository, client *MockBankClient) {
				token := "valid-token"
				consentID := "consent-123"
				expiresAt := time.Now().Add(1 * time.Hour)

				connection := &models.BankConnection{
					ID:             1,
					UserID:         1,
					BankProvider:   models.VBankProvider,
					BankClientID:   "team069-1",
					AccessToken:    &token,
					ConsentID:      &consentID,
					TokenExpiresAt: &expiresAt,
					Status:         models.ConnectionActive,
				}

				repo.On("FindByUserAndBank", ctx, 1, models.VBankProvider).Return(connection, nil)

				balances := []*models.Balance{
					{AccountID: "acc-123", Amount: 10000.50, Currency: "RUB", Type: "InterimAvailable"},
					{AccountID: "acc-123", Amount: 10000.50, Currency: "RUB", Type: "InterimBooked"},
				}
				client.On("GetBalances", ctx, token, "team069-1", consentID, "acc-123").Return(balances, nil)
			},
			expectedError: false,
			validate: func(t *testing.T, balances []*models.Balance) {
				assert.Len(t, balances, 2)
				assert.Equal(t, "acc-123", balances[0].AccountID)
				assert.Equal(t, 10000.50, balances[0].Amount)
				assert.Equal(t, "RUB", balances[0].Currency)
			},
		},
		{
			name: "connection not found",
			request: &GetBalancesRequest{
				UserID:       1,
				BankProvider: models.VBankProvider,
				AccountID:    "acc-123",
			},
			mockSetup: func(repo *MockBankConnectionRepository, client *MockBankClient) {
				repo.On("FindByUserAndBank", ctx, 1, models.VBankProvider).Return(nil, fmt.Errorf("not found"))
			},
			expectedError: true,
		},
		{
			name: "connection not active",
			request: &GetBalancesRequest{
				UserID:       1,
				BankProvider: models.VBankProvider,
				AccountID:    "acc-123",
			},
			mockSetup: func(repo *MockBankConnectionRepository, client *MockBankClient) {
				connection := &models.BankConnection{
					ID:           1,
					UserID:       1,
					BankProvider: models.VBankProvider,
					Status:       models.ConnectionExpired,
				}
				repo.On("FindByUserAndBank", ctx, 1, models.VBankProvider).Return(connection, nil)
			},
			expectedError: true,
		},
		{
			name: "token expired - refresh success",
			request: &GetBalancesRequest{
				UserID:       1,
				BankProvider: models.VBankProvider,
				AccountID:    "acc-123",
			},
			mockSetup: func(repo *MockBankConnectionRepository, client *MockBankClient) {
				oldToken := "expired-token"
				consentID := "consent-123"
				expiredTime := time.Now().Add(-1 * time.Hour)

				connection := &models.BankConnection{
					ID:             1,
					UserID:         1,
					BankProvider:   models.VBankProvider,
					BankClientID:   "team069-1",
					AccessToken:    &oldToken,
					ConsentID:      &consentID,
					TokenExpiresAt: &expiredTime,
					Status:         models.ConnectionActive,
				}

				repo.On("FindByUserAndBank", ctx, 1, models.VBankProvider).Return(connection, nil)

				// Mock token refresh
				newToken := "new-token"
				client.On("GetBankToken", ctx).Return(newToken, int64(3600), nil)
				repo.On("UpdateToken", ctx, 1, newToken, mock.AnythingOfType("time.Time")).Return(nil)

				// Mock GetBalances with new token
				balances := []*models.Balance{
					{AccountID: "acc-123", Amount: 5000.00, Currency: "RUB", Type: "InterimAvailable"},
				}
				client.On("GetBalances", ctx, newToken, "team069-1", consentID, "acc-123").Return(balances, nil)
			},
			expectedError: false,
			validate: func(t *testing.T, balances []*models.Balance) {
				assert.Len(t, balances, 1)
				assert.Equal(t, 5000.00, balances[0].Amount)
			},
		},
		{
			name: "unsupported bank provider",
			request: &GetBalancesRequest{
				UserID:       1,
				BankProvider: models.SBankProvider,
				AccountID:    "acc-123",
			},
			mockSetup: func(repo *MockBankConnectionRepository, client *MockBankClient) {
				token := "valid-token"
				consentID := "consent-123"
				expiresAt := time.Now().Add(1 * time.Hour)

				connection := &models.BankConnection{
					ID:             1,
					UserID:         1,
					BankProvider:   models.SBankProvider,
					BankClientID:   "sbank-client",
					AccessToken:    &token,
					ConsentID:      &consentID,
					TokenExpiresAt: &expiresAt,
					Status:         models.ConnectionActive,
				}

				repo.On("FindByUserAndBank", ctx, 1, models.SBankProvider).Return(connection, nil)
			},
			expectedError: true,
		},
		{
			name: "bank API error",
			request: &GetBalancesRequest{
				UserID:       1,
				BankProvider: models.VBankProvider,
				AccountID:    "acc-123",
			},
			mockSetup: func(repo *MockBankConnectionRepository, client *MockBankClient) {
				token := "valid-token"
				consentID := "consent-123"
				expiresAt := time.Now().Add(1 * time.Hour)

				connection := &models.BankConnection{
					ID:             1,
					UserID:         1,
					BankProvider:   models.VBankProvider,
					BankClientID:   "team069-1",
					AccessToken:    &token,
					ConsentID:      &consentID,
					TokenExpiresAt: &expiresAt,
					Status:         models.ConnectionActive,
				}

				repo.On("FindByUserAndBank", ctx, 1, models.VBankProvider).Return(connection, nil)
				client.On("GetBalances", ctx, token, "team069-1", consentID, "acc-123").
					Return(nil, fmt.Errorf("bank API unavailable"))
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(MockBankConnectionRepository)
			mockClient := new(MockBankClient)

			tt.mockSetup(mockRepo, mockClient)

			service := NewService(mockRepo, map[models.BankProvider]clients.BankClient{
				models.VBankProvider: mockClient,
			}, log)

			balances, err := service.GetBalances(ctx, tt.request)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, balances)
				}
			}

			mockRepo.AssertExpectations(t)
			mockClient.AssertExpectations(t)
		})
	}
}
