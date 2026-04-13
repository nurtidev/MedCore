package damumed

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

// Client реализует adapter.GovAPIAdapter для DAMUMED API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	cb         *adapter.CircuitBreaker
}

// New создаёт DAMUMED клиент.
func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		cb:         adapter.NewCircuitBreaker(),
	}
}

// ValidateIIN проверяет ИИН через DAMUMED (медреестры РК).
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
				fmt.Sprintf("%s/api/patient/by-iin/%s", c.baseURL, iin), nil)
			if err != nil {
				return err
			}
			req.Header.Set("Authorization", "Bearer "+c.apiKey)

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

			var apiResp damumedPatientResponse
			if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
				return fmt.Errorf("damumed: decode response: %w", err)
			}

			birthDate, _ := time.Parse("2006-01-02", apiResp.BirthDate)
			info = &domain.PatientInfo{
				IIN:        iin,
				FirstName:  apiResp.FirstName,
				LastName:   apiResp.LastName,
				MiddleName: apiResp.MiddleName,
				BirthDate:  birthDate,
				Gender:     apiResp.Gender,
				IsValid:    true,
			}
			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("damumed.ValidateIIN: %w", err)
	}
	return info, nil
}

// GetPatientStatus возвращает медицинский статус пациента через DAMUMED.
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
				fmt.Sprintf("%s/api/patient/%s/medical-status", c.baseURL, iin), nil)
			if err != nil {
				return err
			}
			req.Header.Set("Authorization", "Bearer "+c.apiKey)

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
				Status string `json:"medicalStatus"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
				return fmt.Errorf("damumed: decode status response: %w", err)
			}
			statusStr = apiResp.Status
			return nil
		})
	})
	if err != nil {
		return "", fmt.Errorf("damumed.GetPatientStatus: %w", err)
	}
	return statusStr, nil
}

type damumedPatientResponse struct {
	FirstName  string `json:"firstName"`
	LastName   string `json:"lastName"`
	MiddleName string `json:"middleName"`
	BirthDate  string `json:"birthDate"`
	Gender     string `json:"gender"`
}
