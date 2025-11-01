package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/kirill010106/multibank_service_app/backend/internal/models"
)

type BankConnectionRepository interface {
	// Create creates a new bank connection
	Create(ctx context.Context, conn *models.BankConnection) error

	// FindByUserAndBank finds a bank connection by user ID and bank provider
	FindByUserAndBank(ctx context.Context, userId int, provider models.BankProvider) (*models.BankConnection, error)

	// FindAllByUser retrieves all bank connections for a given user ID
	FindAllByUser(ctx context.Context, userId int) ([]*models.BankConnection, error)

	// UpdateToken updates the token and expiration time for a bank connection
	UpdateToken(ctx context.Context, id int, token string, expiresAt time.Time) error

	// UpdateConsent updates the consent ID and status for a bank connection
	UpdateConsent(ctx context.Context, id int, consentID string, status models.ConnectionStatus) error

	// UpdateStatus updates the status of a bank connection
	UpdateStatus(ctx context.Context, id int, status models.ConnectionStatus) error

	// Delete deletes a bank connection by its ID (soft delete via status)
	Delete(ctx context.Context, id int) error
}

type PostgresBankConnectionRepository struct {
	db *sql.DB
}

func NewBankConnectionRepository(db *sql.DB) BankConnectionRepository {
	return &PostgresBankConnectionRepository{db: db}
}

func (r *PostgresBankConnectionRepository) Create(ctx context.Context, conn *models.BankConnection) error {
	const op = "repository.BankConnection.Create"

	query := `
		INSERT INTO bank_connections
		(user_id, bank_provider, bank_client_id, consent_id, access_token, token_expires_at, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at`

	err := r.db.QueryRowContext(
		ctx,
		query,
		conn.UserID,
		conn.BankProvider,
		conn.BankClientID,
		conn.ConsentID,
		conn.AccessToken,
		conn.TokenExpiresAt,
		conn.Status,
	).Scan(&conn.ID, &conn.CreatedAt, &conn.UpdatedAt)

	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (r *PostgresBankConnectionRepository) FindByUserAndBank(ctx context.Context, userID int, provider models.BankProvider) (*models.BankConnection, error) {
	const op = "repository.BankConnection.FindByUserAndBank"

	query := `
		SELECT id, user_id, bank_provider, bank_client_id, consent_id, access_token, token_expires_at, status, created_at, updated_at
		FROM bank_connections
		WHERE user_id = $1 AND bank_provider = $2 AND status != 'revoked'
		ORDER BY created_at DESC
		LIMIT 1`

	conn := &models.BankConnection{}

	err := r.db.QueryRowContext(ctx, query, userID, provider).Scan(
		&conn.ID,
		&conn.UserID,
		&conn.BankProvider,
		&conn.BankClientID,
		&conn.ConsentID,
		&conn.AccessToken,
		&conn.TokenExpiresAt,
		&conn.Status,
		&conn.CreatedAt,
		&conn.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%s: connection not found", op)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return conn, nil
}

func (r *PostgresBankConnectionRepository) FindAllByUser(ctx context.Context, userID int) ([]*models.BankConnection, error) {
	const op = "repository.BankConnection.FindAllByUser"

	query := `
		SELECT id, user_id, bank_provider, bank_client_id, consent_id, access_token, token_expires_at, status, created_at, updated_at
		FROM bank_connections
		WHERE user_id = $1 AND status != 'revoked'
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	defer rows.Close()

	connections := []*models.BankConnection{}

	for rows.Next() {
		conn := &models.BankConnection{}
		err := rows.Scan(
			&conn.ID,
			&conn.UserID,
			&conn.BankProvider,
			&conn.BankClientID,
			&conn.ConsentID,
			&conn.AccessToken,
			&conn.TokenExpiresAt,
			&conn.Status,
			&conn.CreatedAt,
			&conn.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("%s: scan error: %w", op, err)
		}
		connections = append(connections, conn)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows error: %w", op, err)
	}

	return connections, nil
}

func (r *PostgresBankConnectionRepository) UpdateToken(ctx context.Context, id int, token string, expiresAt time.Time) error {
	const op = "repository.BankConnection.UpdateToken"

	query := `
		UPDATE bank_connections
		SET access_token = $1, token_expires_at = $2, updated_at = NOW()
		WHERE id = $3`

	result, err := r.db.ExecContext(ctx, query, token, expiresAt, id)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: rows affected error: %w", op, err)
	}
	if rows == 0 {
		return fmt.Errorf("%s: connection not found", op)
	}
	return nil
}

func (r *PostgresBankConnectionRepository) UpdateConsent(ctx context.Context, id int, consentID string, status models.ConnectionStatus) error {
	const op = "repository.BankConnection.UpdateConsent"

	query := `
		UPDATE bank_connections
		SET consent_id = $1, status = $2, updated_at = NOW()
		WHERE id = $3`

	result, err := r.db.ExecContext(ctx, query, consentID, status, id)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: rows affected error: %w", op, err)
	}

	if rows == 0 {
		return fmt.Errorf("%s: connection not found", op)
	}

	return nil
}

func (r *PostgresBankConnectionRepository) UpdateStatus(ctx context.Context, id int, status models.ConnectionStatus) error {
	const op = "repository.BankConnection.UpdateStatus"

	query := `
		UPDATE bank_connections
		SET status = $1, updated_at = NOW()
		WHERE id = $2`

	result, err := r.db.ExecContext(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: rows affected error: %w", op, err)
	}

	if rows == 0 {
		return fmt.Errorf("%s: connection not found", op)
	}

	return nil
}

func (r *PostgresBankConnectionRepository) Delete(ctx context.Context, id int) error {
	const op = "repository.BankConnection.Delete"
	return r.UpdateStatus(ctx, id, models.ConnectionRevoked)
}
