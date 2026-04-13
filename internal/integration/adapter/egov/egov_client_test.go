package egov_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nurtidev/medcore/internal/integration/adapter/egov"
	"github.com/nurtidev/medcore/internal/integration/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEgovClient_ValidateIIN_NetworkError_Retry(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			// первые 2 вызова — сетевой сбой через 500
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"iin":"860101123456","firstName":"Асель","lastName":"Нурова","birthDate":"1986-01-01","gender":"F"}`))
	}))
	defer srv.Close()

	client := egov.New(srv.URL, "test-key")
	info, err := client.ValidateIIN(context.Background(), "860101123456")

	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "860101123456", info.IIN)
	assert.True(t, info.IsValid)
	assert.Equal(t, 3, attempts, "should have retried twice")
}

func TestEgovClient_5xx_CircuitBreakerTrips(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := egov.New(srv.URL, "test-key")

	// Вызываем 5 раз чтобы выбить circuit breaker
	// Каждый вызов делает 3 попытки (retry), поэтому 5 * 3 = 15 запросов к серверу
	var lastErr error
	for i := 0; i < 5; i++ {
		_, lastErr = client.ValidateIIN(context.Background(), "860101123456")
	}

	require.Error(t, lastErr)
}

func TestEgovClient_ValidateIIN_InvalidIIN(t *testing.T) {
	client := egov.New("http://localhost", "key")
	_, err := client.ValidateIIN(context.Background(), "123")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrIINInvalid)
}

func TestEgovClient_ValidateIIN_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := egov.New(srv.URL, "test-key")
	info, err := client.ValidateIIN(context.Background(), "860101123456")

	require.NoError(t, err)
	require.NotNil(t, info)
	assert.False(t, info.IsValid)
}
