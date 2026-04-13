package adapter

import (
	"errors"
	"sync"
	"time"

	"github.com/nurtidev/medcore/internal/integration/domain"
)

// cbState — состояние автоматического выключателя.
type cbState int

const (
	cbClosed   cbState = iota // нормальная работа
	cbOpen                    // запросы блокируются
	cbHalfOpen                // пробный запрос
)

// CircuitBreaker — простой circuit breaker с тремя состояниями.
// MaxRequests: максимум успешных запросов в Half-Open до перехода в Closed.
// Timeout: время в Open состоянии перед переходом в Half-Open.
// ReadyToTrip: количество подряд идущих ошибок для перехода в Open.
type CircuitBreaker struct {
	mu           sync.Mutex
	state        cbState
	failures     int
	successes    int
	lastFailedAt time.Time

	MaxRequests int
	Timeout     time.Duration
	ReadyToTrip int
}

// NewCircuitBreaker создаёт circuit breaker с настройками из ТЗ:
// 3 запроса в half-open, 30s таймаут, 5 последовательных ошибок.
func NewCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{
		MaxRequests: 3,
		Timeout:     30 * time.Second,
		ReadyToTrip: 5,
	}
}

// Execute выполняет fn через circuit breaker.
func (cb *CircuitBreaker) Execute(fn func() error) error {
	if err := cb.allow(); err != nil {
		return err
	}

	err := fn()
	cb.record(err)
	return err
}

func (cb *CircuitBreaker) allow() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case cbOpen:
		if time.Since(cb.lastFailedAt) > cb.Timeout {
			cb.state = cbHalfOpen
			cb.successes = 0
			return nil
		}
		return domain.ErrCircuitBreakerOpen
	case cbHalfOpen:
		if cb.successes >= cb.MaxRequests {
			return domain.ErrCircuitBreakerOpen
		}
		return nil
	default:
		return nil
	}
}

func (cb *CircuitBreaker) record(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failures++
		cb.lastFailedAt = time.Now()
		if cb.state == cbHalfOpen || cb.failures >= cb.ReadyToTrip {
			cb.state = cbOpen
			cb.failures = 0
		}
		return
	}

	// success
	cb.failures = 0
	if cb.state == cbHalfOpen {
		cb.successes++
		if cb.successes >= cb.MaxRequests {
			cb.state = cbClosed
			cb.successes = 0
		}
	}
}

// IsOpen возвращает true если выключатель разомкнут.
func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state == cbOpen && time.Since(cb.lastFailedAt) <= cb.Timeout
}

// isRetryable возвращает true для сетевых ошибок и 5xx (но не 4xx).
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode >= 500
	}
	return true
}

// HTTPError — ошибка с HTTP статус кодом от внешнего API.
type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	return "http error " + string(rune(e.StatusCode)) + ": " + e.Body
}
