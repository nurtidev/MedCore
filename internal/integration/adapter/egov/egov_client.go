package egov

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/nurtidev/medcore/internal/integration/adapter"
	"github.com/nurtidev/medcore/internal/integration/domain"
	"github.com/sethvargo/go-retry"
)

// Client реализует adapter.GovAPIAdapter для eGov API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	cb         *adapter.CircuitBreaker
}

// New создаёт eGov клиент.
func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		cb:         adapter.NewCircuitBreaker(),
	}
}

// ValidateIIN проверяет ИИН через eGov API.
func (c *Client) ValidateIIN(ctx context.Context, iin string) (*domain.PatientInfo, error) {
	if len(iin) != 12 {
		return nil, domain.ErrIINInvalid
	}

	var info *domain.PatientInfo
	err := c.cb.Execute(func() error {
		return retry.Do(ctx, retry.WithMaxRetries(3, retry.NewExponential(100*time.Millisecond)), func(ctx context.Context) error {
			callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			req, err := http.NewRequestWithContext(callCtx, http.MethodGet,
				fmt.Sprintf("%s/api/v1/citizens/%s", c.baseURL, iin), nil)
			if err != nil {
				return err
			}
			req.Header.Set("X-API-Key", c.apiKey)

			resp, err := c.httpClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				info = &domain.PatientInfo{IIN: iin, IsValid: false}
				return nil
			}
			if resp.StatusCode >= 400 {
				body, _ := io.ReadAll(resp.Body)
				httpErr := &adapter.HTTPError{StatusCode: resp.StatusCode, Body: string(body)}
				if resp.StatusCode >= 500 {
					return retry.RetryableError(httpErr)
				}
				return httpErr
			}

			var apiResp egovCitizenResponse
			if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
				return fmt.Errorf("egov: decode response: %w", err)
			}

			info = mapToPatientInfo(iin, &apiResp)
			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("egov.ValidateIIN: %w", err)
	}
	return info, nil
}

// GetPatientStatus возвращает статус пациента из eGov API.
func (c *Client) GetPatientStatus(ctx context.Context, iin string) (string, error) {
	if len(iin) != 12 {
		return "", domain.ErrIINInvalid
	}

	var statusStr string
	err := c.cb.Execute(func() error {
		return retry.Do(ctx, retry.WithMaxRetries(3, retry.NewExponential(100*time.Millisecond)), func(ctx context.Context) error {
			callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			req, err := http.NewRequestWithContext(callCtx, http.MethodGet,
				fmt.Sprintf("%s/api/v1/citizens/%s/status", c.baseURL, iin), nil)
			if err != nil {
				return err
			}
			req.Header.Set("X-API-Key", c.apiKey)

			resp, err := c.httpClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode >= 400 {
				body, _ := io.ReadAll(resp.Body)
				httpErr := &adapter.HTTPError{StatusCode: resp.StatusCode, Body: string(body)}
				if resp.StatusCode >= 500 {
					return retry.RetryableError(httpErr)
				}
				return httpErr
			}

			var apiResp struct {
				Status string `json:"status"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
				return fmt.Errorf("egov: decode status response: %w", err)
			}
			statusStr = apiResp.Status
			return nil
		})
	})
	if err != nil {
		return "", fmt.Errorf("egov.GetPatientStatus: %w", err)
	}
	return statusStr, nil
}

// ── Internal types ────────────────────────────────────────────────────────────

type egovCitizenResponse struct {
	IIN        string `json:"iin"`
	FirstName  string `json:"firstName"`
	LastName   string `json:"lastName"`
	MiddleName string `json:"middleName"`
	BirthDate  string `json:"birthDate"` // "YYYY-MM-DD"
	Gender     string `json:"gender"`
	Address    string `json:"address"`
}

func mapToPatientInfo(iin string, r *egovCitizenResponse) *domain.PatientInfo {
	birthDate, _ := time.Parse("2006-01-02", r.BirthDate)
	return &domain.PatientInfo{
		IIN:        iin,
		FirstName:  r.FirstName,
		LastName:   r.LastName,
		MiddleName: r.MiddleName,
		BirthDate:  birthDate,
		Gender:     r.Gender,
		Address:    r.Address,
		IsValid:    true,
	}
}
