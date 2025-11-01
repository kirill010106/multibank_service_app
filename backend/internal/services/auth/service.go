package auth

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/kirill010106/multibank_service_app/backend/internal/database"
	"github.com/kirill010106/multibank_service_app/backend/internal/models"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	db         database.DBInterface // ← ИЗМЕНЕНО: интерфейс вместо *sql.DB
	jwtManager *JWTManager
}

// NewService - создаем сервис с реальной БД
func NewService(db *sql.DB, jwtManager *JWTManager) *Service {
	return &Service{
		db:         database.NewSQLDB(db), // ← ИЗМЕНЕНО: оборачиваем в интерфейс
		jwtManager: jwtManager,
	}
}

// NewServiceWithDB - создаем сервис с любой имплементацией DBInterface (для тестов)
func NewServiceWithDB(db database.DBInterface, jwtManager *JWTManager) *Service {
	return &Service{
		db:         db,
		jwtManager: jwtManager,
	}
}

func (s *Service) Register(req *models.RegisterRequest) (*models.AuthResponse, error) {
	const op = "auth.Service.Register"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var exists bool

	err := s.db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)",
		req.Email,
	).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to check email: %w", op, err)
	}
	if exists {
		return nil, fmt.Errorf("%s: email already registered", op)
	}

	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte(req.Password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to hash password: %w", op, err)
	}

	var user models.User

	err = s.db.QueryRowContext(ctx, `
	INSERT INTO users (email, password_hash, created_at, updated_at)
	VALUES ($1, $2, NOW(), NOW())
	RETURNING id, email, created_at, updated_at
	`, req.Email, string(passwordHash)).Scan(
		&user.ID,
		&user.Email,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to create user: %w", op, err)
	}

	token, err := s.jwtManager.GenerateToken(user.ID, user.Email)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to generate token: %w", op, err)
	}

	return &models.AuthResponse{
		Token: token,
		User:  &user,
	}, nil
}

func (s *Service) Login(req *models.LoginRequest) (*models.AuthResponse, error) {
	const op = "auth.Service.Login"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var user models.User

	// Query user from database
	err := s.db.QueryRowContext(ctx, `
        SELECT id, email, password_hash, created_at, updated_at
        FROM users
        WHERE email = $1
    `, req.Email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			// Domain error - return without wrapping
			// Handler will check with errors.Is()
			return nil, models.ErrInvalidCredentials
		}
		// Real database error - wrap with context
		return nil, fmt.Errorf("%s: database query failed: %w", op, err)
	}

	// Verify password
	err = bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash),
		[]byte(req.Password),
	)
	if err != nil {
		// Domain error - return without wrapping
		return nil, models.ErrInvalidCredentials
	}

	// Generate JWT token
	token, err := s.jwtManager.GenerateToken(user.ID, user.Email)
	if err != nil {
		return nil, fmt.Errorf("%s: token generation failed: %w", op, err)
	}

	return &models.AuthResponse{
		Token: token,
		User:  &user,
	}, nil
}
