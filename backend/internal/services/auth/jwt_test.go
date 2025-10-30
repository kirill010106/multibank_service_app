package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "test-secret-key-minimum-32-chars-for-security"

func TestNewJWTManager(t *testing.T) {
	duration := 24 * time.Hour
	manager := NewJWTManager(testSecret, duration)

	assert.NotNil(t, manager)
	assert.Equal(t, testSecret, manager.secretKey)
	assert.Equal(t, duration, manager.tokenDuration)
}

func TestJWTManager_GenerateToken_Success(t *testing.T) {
	manager := NewJWTManager(testSecret, 1*time.Hour)

	token, err := manager.GenerateToken(42, "test@example.com")

	require.NoError(t, err)
	require.NotEmpty(t, token)

	// Токен должен состоять из 3 частей (header.payload.signature)
	parts := strings.Split(token, ".")
	assert.Len(t, parts, 3, "JWT должен иметь 3 части")
}

func TestJWTManager_GenerateToken_MultipleUsers(t *testing.T) {
	manager := NewJWTManager(testSecret, 1*time.Hour)

	token1, err1 := manager.GenerateToken(1, "user1@example.com")
	token2, err2 := manager.GenerateToken(2, "user2@example.com")

	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.NotEqual(t, token1, token2, "Токены для разных юзеров должны отличаться")
}

func TestJWTManager_ValidateToken_Success(t *testing.T) {
	manager := NewJWTManager(testSecret, 1*time.Hour)

	expectedUserID := 42
	expectedEmail := "test@example.com"

	// Генерируем токен
	token, err := manager.GenerateToken(expectedUserID, expectedEmail)
	require.NoError(t, err)

	// Валидируем токен
	claims, err := manager.ValidateToken(token)

	require.NoError(t, err)
	require.NotNil(t, claims)
	assert.Equal(t, expectedUserID, claims.UserID)
	assert.Equal(t, expectedEmail, claims.Email)

	// Проверяем что ExpiresAt установлен
	assert.NotNil(t, claims.ExpiresAt)
	assert.True(t, claims.ExpiresAt.After(time.Now()), "ExpiresAt должен быть в будущем")
}

func TestJWTManager_ValidateToken_InvalidFormat(t *testing.T) {
	manager := NewJWTManager(testSecret, 1*time.Hour)

	testCases := []struct {
		name  string
		token string
	}{
		{
			name:  "пустой токен",
			token: "",
		},
		{
			name:  "случайная строка",
			token: "this-is-not-a-valid-jwt-token",
		},
		{
			name:  "неполный токен (2 части)",
			token: "header.payload",
		},
		{
			name:  "токен с лишними частями",
			token: "header.payload.signature.extra",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := manager.ValidateToken(tc.token)
			assert.Error(t, err, "Невалидный токен должен возвращать ошибку")
		})
	}
}

func TestJWTManager_ValidateToken_WrongSecret(t *testing.T) {
	manager1 := NewJWTManager("secret-key-one-min-32-chars-length", 1*time.Hour)
	manager2 := NewJWTManager("secret-key-two-min-32-chars-length", 1*time.Hour)

	// Генерируем токен с одним секретом
	token, err := manager1.GenerateToken(42, "test@example.com")
	require.NoError(t, err)

	// Пытаемся валидировать с другим секретом
	_, err = manager2.ValidateToken(token)
	assert.Error(t, err, "Токен подписанный другим ключом должен быть невалидным")
}

func TestJWTManager_ValidateToken_ExpiredToken(t *testing.T) {
	// Создаем токен который протухнет через 1 миллисекунду
	manager := NewJWTManager(testSecret, 1*time.Millisecond)

	token, err := manager.GenerateToken(42, "test@example.com")
	require.NoError(t, err)

	// Ждем пока токен протухнет
	time.Sleep(10 * time.Millisecond)

	// Пытаемся валидировать протухший токен
	_, err = manager.ValidateToken(token)
	assert.Error(t, err, "Протухший токен должен возвращать ошибку")
	assert.Contains(t, err.Error(), "expired", "Ошибка должна содержать 'expired'")
}

func TestJWTManager_ValidateToken_TamperedPayload(t *testing.T) {
	manager := NewJWTManager(testSecret, 1*time.Hour)

	token, err := manager.GenerateToken(42, "test@example.com")
	require.NoError(t, err)

	// Изменяем payload (меняем user_id)
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)

	// Подделываем payload (меняем claims)
	tamperedClaims := Claims{
		UserID: 999, // было 42
		Email:  "hacker@example.com",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}
	tamperedToken := jwt.NewWithClaims(jwt.SigningMethodHS256, tamperedClaims)
	tamperedString, _ := tamperedToken.SignedString([]byte("wrong-secret"))

	// Пытаемся валидировать подделанный токен
	_, err = manager.ValidateToken(tamperedString)
	assert.Error(t, err, "Подделанный токен должен быть невалидным")
}

func TestJWTManager_ValidateToken_AlgorithmNone(t *testing.T) {
	manager := NewJWTManager(testSecret, 1*time.Hour)

	// Создаем токен с алгоритмом "none" (попытка атаки)
	claims := Claims{
		UserID: 42,
		Email:  "test@example.com",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}

	// Токен без подписи (алгоритм none)
	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	noneToken, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	// Пытаемся валидировать токен без подписи
	_, err = manager.ValidateToken(noneToken)
	assert.Error(t, err, "Токен с алгоритмом 'none' должен быть отклонен")
	assert.Contains(t, err.Error(), "signing method", "Ошибка должна упоминать signing method")
}

func TestJWTManager_TokenRoundtrip(t *testing.T) {
	manager := NewJWTManager(testSecret, 24*time.Hour)

	testCases := []struct {
		userID int
		email  string
	}{
		{1, "user1@example.com"},
		{999, "admin@example.com"},
		{12345, "test.user+tag@domain.co.uk"},
	}

	for _, tc := range testCases {
		t.Run(tc.email, func(t *testing.T) {
			// Generate
			token, err := manager.GenerateToken(tc.userID, tc.email)
			require.NoError(t, err)

			// Validate
			claims, err := manager.ValidateToken(token)
			require.NoError(t, err)

			// Verify
			assert.Equal(t, tc.userID, claims.UserID)
			assert.Equal(t, tc.email, claims.Email)
		})
	}
}

func TestJWTManager_ConcurrentGeneration(t *testing.T) {
	manager := NewJWTManager(testSecret, 1*time.Hour)

	const numGoroutines = 100
	tokens := make(chan string, numGoroutines)

	// Генерируем токены параллельно
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			token, err := manager.GenerateToken(id, "user@example.com")
			require.NoError(t, err)
			tokens <- token
		}(i)
	}

	// Собираем результаты
	tokenSet := make(map[string]bool)
	for i := 0; i < numGoroutines; i++ {
		token := <-tokens
		tokenSet[token] = true
	}

	// Все токены должны быть уникальными
	assert.Len(t, tokenSet, numGoroutines, "Все токены должны быть уникальными")
}

func BenchmarkJWTManager_GenerateToken(b *testing.B) {
	manager := NewJWTManager(testSecret, 1*time.Hour)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := manager.GenerateToken(42, "test@example.com")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkJWTManager_ValidateToken(b *testing.B) {
	manager := NewJWTManager(testSecret, 1*time.Hour)
	token, _ := manager.GenerateToken(42, "test@example.com")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := manager.ValidateToken(token)
		if err != nil {
			b.Fatal(err)
		}
	}
}
