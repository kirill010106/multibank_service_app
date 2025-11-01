package clients

import (
	"context"

	"github.com/kirill010106/multibank_service_app/backend/internal/models"
)

type BankClient interface {
	GetBankToken(ctx context.Context) (string, int64, error)

	CreateConsent(ctx context.Context, bankToken, bankClientID string) (string, error)

	GetAccounts(ctx context.Context, bankToken, bankClientID, consentID string) ([]*models.Account, error)

	GetBankProvider() models.BankProvider
}
