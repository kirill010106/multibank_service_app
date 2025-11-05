package auth

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kirill010106/multibank_service_app/backend/internal/mocks"
	"github.com/kirill010106/multibank_service_app/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

const testJWTSecret = "test-jwt-secret-key-minimum-32-characters-long"

// setupTestService создает сервис с моками для тестирования
func setupTestService(t *testing.T) (*Service, *mocks.MockDBInterface) {
	mockDB := mocks.NewMockDBInterface(t)
	jwtManager := NewJWTManager(testJWTSecret, 24*time.Hour)
	service := NewServiceWithDB(mockDB, jwtManager)
	return service, mockDB
}

// TestService_Register_Success тестирует успешную регистрацию
func TestService_Register_Success(t *testing.T) {
	service, mockDB := setupTestService(t)

	email := "test@example.com"
	password := "qwerty123"
	req := &models.RegisterRequest{
		Email:    email,
		Password: password,
	}

	// Мок для проверки существования email
	mockRowScanner := mocks.NewMockRowScanner(t)
	mockRowScanner.EXPECT().
		Scan(mock.AnythingOfType("*bool")).
		Run(func(dest ...interface{}) {
			// Email НЕ существует
			exists := dest[0].(*bool)
			*exists = false
		}).
		Return(nil).
		Once()

	mockDB.EXPECT().
		QueryRowContext(
			mock.AnythingOfType("*context.timerCtx"),
			"SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)",
			email,
		).
		Return(mockRowScanner).
		Once()

	// Мок для INSERT нового юзера
	mockInsertRow := mocks.NewMockRowScanner(t)
	mockInsertRow.EXPECT().
		Scan(
			mock.AnythingOfType("*int"),       // id
			mock.AnythingOfType("*string"),    // email
			mock.AnythingOfType("*time.Time"), // created_at
			mock.AnythingOfType("*time.Time"), // updated_at
		).
		Run(func(dest ...interface{}) {
			// Эмулируем возврат данных из БД
			id := dest[0].(*int)
			email := dest[1].(*string)
			createdAt := dest[2].(*time.Time)
			updatedAt := dest[3].(*time.Time)

			*id = 1
			*email = req.Email
			*createdAt = time.Now()
			*updatedAt = time.Now()
		}).
		Return(nil).
		Once()

	mockDB.EXPECT().
		QueryRowContext(
			mock.AnythingOfType("*context.timerCtx"),
			mock.MatchedBy(func(query string) bool {
				// Проверяем что это INSERT запрос (убираем пробельные символы для надёжности)
				normalized := strings.Join(strings.Fields(query), " ")
				return strings.Contains(strings.ToUpper(normalized), "INSERT INTO USERS")
			}),
			email,
			mock.AnythingOfType("string"), // password hash
		).
		Return(mockInsertRow).
		Once()

	// Вызываем метод
	response, err := service.Register(req)

	// Проверки
	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, 1, response.User.ID)
	assert.Equal(t, email, response.User.Email)
	assert.NotEmpty(t, response.Token)

	// Проверяем что все моки вызвались
	mockDB.AssertExpectations(t)
	mockRowScanner.AssertExpectations(t)
	mockInsertRow.AssertExpectations(t)
}

// TestService_Register_EmailAlreadyExists тестирует регистрацию с существующим email
func TestService_Register_EmailAlreadyExists(t *testing.T) {
	service, mockDB := setupTestService(t)

	req := &models.RegisterRequest{
		Email:    "existing@example.com",
		Password: "password123",
	}

	// Мок: email УЖЕ существует
	mockRowScanner := mocks.NewMockRowScanner(t)
	mockRowScanner.EXPECT().
		Scan(mock.AnythingOfType("*bool")).
		Run(func(dest ...interface{}) {
			exists := dest[0].(*bool)
			*exists = true // ← email существует
		}).
		Return(nil).
		Once()

	mockDB.EXPECT().
		QueryRowContext(
			mock.AnythingOfType("*context.timerCtx"),
			"SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)",
			req.Email,
		).
		Return(mockRowScanner).
		Once()

	// Вызываем метод
	response, err := service.Register(req)

	// Проверки
	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "email already registered")

	mockDB.AssertExpectations(t)
	mockRowScanner.AssertExpectations(t)
}

// TestService_Register_DatabaseError тестирует обработку ошибки БД
func TestService_Register_DatabaseError(t *testing.T) {
	service, mockDB := setupTestService(t)

	req := &models.RegisterRequest{
		Email:    "test@example.com",
		Password: "password123",
	}

	// Мок: БД вернула ошибку
	mockRowScanner := mocks.NewMockRowScanner(t)
	mockRowScanner.EXPECT().
		Scan(mock.AnythingOfType("*bool")).
		Return(sql.ErrConnDone). // ← ошибка БД
		Once()

	mockDB.EXPECT().
		QueryRowContext(
			mock.AnythingOfType("*context.timerCtx"),
			"SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)",
			req.Email,
		).
		Return(mockRowScanner).
		Once()

	// Вызываем метод
	response, err := service.Register(req)

	// Проверки
	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "failed to check email")

	mockDB.AssertExpectations(t)
}

// TestService_Login_Success тестирует успешный логин
func TestService_Login_Success(t *testing.T) {
	service, mockDB := setupTestService(t)

	email := "test@example.com"
	password := "qwerty123"
	passwordHash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	req := &models.LoginRequest{
		Email:    email,
		Password: password,
	}

	// Мок: находим юзера в БД
	mockRowScanner := mocks.NewMockRowScanner(t)
	mockRowScanner.EXPECT().
		Scan(
			mock.AnythingOfType("*int"),       // id
			mock.AnythingOfType("*string"),    // email
			mock.AnythingOfType("*string"),    // password_hash
			mock.AnythingOfType("*time.Time"), // created_at
			mock.AnythingOfType("*time.Time"), // updated_at
		).
		Run(func(dest ...interface{}) {
			// Возвращаем данные юзера
			id := dest[0].(*int)
			emailPtr := dest[1].(*string)
			hashPtr := dest[2].(*string)
			createdAt := dest[3].(*time.Time)
			updatedAt := dest[4].(*time.Time)

			*id = 42
			*emailPtr = email
			*hashPtr = string(passwordHash)
			*createdAt = time.Now().Add(-24 * time.Hour)
			*updatedAt = time.Now()
		}).
		Return(nil).
		Once()

	mockDB.EXPECT().
		QueryRowContext(
			mock.AnythingOfType("*context.timerCtx"),
			mock.MatchedBy(func(query string) bool {
				// Проверяем что это SELECT запрос для логина
				return strings.Contains(strings.ToUpper(query), "SELECT") &&
					strings.Contains(strings.ToUpper(query), "FROM USERS") &&
					strings.Contains(strings.ToUpper(query), "WHERE EMAIL")
			}),
			email,
		).
		Return(mockRowScanner).
		Once()

	// Вызываем метод
	response, err := service.Login(req)

	// Проверки
	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, 42, response.User.ID)
	assert.Equal(t, email, response.User.Email)
	assert.NotEmpty(t, response.Token)

	mockDB.AssertExpectations(t)
	mockRowScanner.AssertExpectations(t)
}

// TestService_Login_UserNotFound тестирует логин с несуществующим email
func TestService_Login_UserNotFound(t *testing.T) {
	service, mockDB := setupTestService(t)

	req := &models.LoginRequest{
		Email:    "nonexistent@example.com",
		Password: "password123",
	}

	// Мок: юзер не найден (sql.ErrNoRows)
	mockRowScanner := mocks.NewMockRowScanner(t)
	mockRowScanner.EXPECT().
		Scan(
			mock.AnythingOfType("*int"),
			mock.AnythingOfType("*string"),
			mock.AnythingOfType("*string"),
			mock.AnythingOfType("*time.Time"),
			mock.AnythingOfType("*time.Time"),
		).
		Return(sql.ErrNoRows). // ← юзер не найден
		Once()

	mockDB.EXPECT().
		QueryRowContext(
			mock.AnythingOfType("*context.timerCtx"),
			mock.MatchedBy(func(query string) bool {
				return strings.Contains(strings.ToUpper(query), "SELECT") && strings.Contains(strings.ToUpper(query), "FROM USERS") && strings.Contains(strings.ToUpper(query), "WHERE EMAIL")
			}),
			req.Email,
		).
		Return(mockRowScanner).
		Once()

	// Вызываем метод
	response, err := service.Login(req)

	// Проверки
	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid credentials")

	mockDB.AssertExpectations(t)
}

// TestService_Login_WrongPassword тестирует логин с неправильным паролем
func TestService_Login_WrongPassword(t *testing.T) {
	service, mockDB := setupTestService(t)

	email := "test@example.com"
	correctPassword := "correct-password"
	wrongPassword := "wrong-password"

	// Хешируем ПРАВИЛЬНЫЙ пароль (который в БД)
	passwordHash, _ := bcrypt.GenerateFromPassword([]byte(correctPassword), bcrypt.DefaultCost)

	req := &models.LoginRequest{
		Email:    email,
		Password: wrongPassword, // ← НЕПРАВИЛЬНЫЙ пароль
	}

	// Мок: находим юзера с правильным хешем
	mockRowScanner := mocks.NewMockRowScanner(t)
	mockRowScanner.EXPECT().
		Scan(
			mock.AnythingOfType("*int"),
			mock.AnythingOfType("*string"),
			mock.AnythingOfType("*string"),
			mock.AnythingOfType("*time.Time"),
			mock.AnythingOfType("*time.Time"),
		).
		Run(func(dest ...interface{}) {
			id := dest[0].(*int)
			emailPtr := dest[1].(*string)
			hashPtr := dest[2].(*string)
			createdAt := dest[3].(*time.Time)
			updatedAt := dest[4].(*time.Time)

			*id = 42
			*emailPtr = email
			*hashPtr = string(passwordHash) // ← хеш ПРАВИЛЬНОГО пароля
			*createdAt = time.Now()
			*updatedAt = time.Now()
		}).
		Return(nil).
		Once()

	mockDB.EXPECT().
		QueryRowContext(
			mock.AnythingOfType("*context.timerCtx"),
			mock.MatchedBy(func(query string) bool {
				return strings.Contains(strings.ToUpper(query), "SELECT") && strings.Contains(strings.ToUpper(query), "FROM USERS") && strings.Contains(strings.ToUpper(query), "WHERE EMAIL")
			}),
			email,
		).
		Return(mockRowScanner).
		Once()

	// Вызываем метод
	response, err := service.Login(req)

	// Проверки: bcrypt.CompareHashAndPassword вернет ошибку
	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid credentials")

	mockDB.AssertExpectations(t)
}

// TestService_Login_DatabaseError тестирует обработку ошибки БД при логине
func TestService_Login_DatabaseError(t *testing.T) {
	service, mockDB := setupTestService(t)

	req := &models.LoginRequest{
		Email:    "test@example.com",
		Password: "password123",
	}

	// Мок: БД вернула ошибку (не sql.ErrNoRows)
	dbError := errors.New("connection timeout")
	mockRowScanner := mocks.NewMockRowScanner(t)
	mockRowScanner.EXPECT().
		Scan(
			mock.AnythingOfType("*int"),
			mock.AnythingOfType("*string"),
			mock.AnythingOfType("*string"),
			mock.AnythingOfType("*time.Time"),
			mock.AnythingOfType("*time.Time"),
		).
		Return(dbError). // ← ошибка БД
		Once()

	mockDB.EXPECT().
		QueryRowContext(
			mock.AnythingOfType("*context.timerCtx"),
			mock.MatchedBy(func(query string) bool {
				return strings.Contains(strings.ToUpper(query), "SELECT") && strings.Contains(strings.ToUpper(query), "FROM USERS") && strings.Contains(strings.ToUpper(query), "WHERE EMAIL")
			}),
			req.Email,
		).
		Return(mockRowScanner).
		Once()

	// Вызываем метод
	response, err := service.Login(req)

	// Проверки
	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "database query failed")
	assert.Contains(t, err.Error(), "connection timeout")

	mockDB.AssertExpectations(t)
}

// TestService_Register_ContextTimeout тестирует таймаут контекста
func TestService_Register_ContextTimeout(t *testing.T) {
	service, mockDB := setupTestService(t)

	req := &models.RegisterRequest{
		Email:    "test@example.com",
		Password: "password123",
	}

	// Мок: возвращаем ошибку контекста
	mockRowScanner := mocks.NewMockRowScanner(t)
	mockRowScanner.EXPECT().
		Scan(mock.AnythingOfType("*bool")).
		Return(context.DeadlineExceeded). // ← таймаут
		Once()

	mockDB.EXPECT().
		QueryRowContext(
			mock.AnythingOfType("*context.timerCtx"),
			"SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)",
			req.Email,
		).
		Return(mockRowScanner).
		Once()

	// Вызываем метод
	response, err := service.Register(req)

	// Проверки
	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "failed to check email")

	mockDB.AssertExpectations(t)
}
