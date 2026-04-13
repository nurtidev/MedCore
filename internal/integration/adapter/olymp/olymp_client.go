package olymp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/nurtidev/medcore/internal/integration/adapter"
	"github.com/nurtidev/medcore/internal/integration/domain"
	"github.com/sethvargo/go-retry"
)

// Client реализует adapter.LaboratoryAdapter для лаборатории Олимп.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	cb         *adapter.CircuitBreaker
}

// New создаёт клиент лаборатории Олимп.
func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		cb:         adapter.NewCircuitBreaker(),
	}
}

// GetPendingResults возвращает необработанные результаты для клиники.
func (c *Client) GetPendingResults(ctx context.Context, clinicID string) ([]*domain.LabResult, error) {
	var results []*domain.LabResult
	err := c.cb.Execute(func() error {
		return retry.Do(ctx, retry.WithMaxRetries(3, retry.NewExponential(100*time.Millisecond)), func(ctx context.Context) error {
			callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			req, err := http.NewRequestWithContext(callCtx, http.MethodGet,
				fmt.Sprintf("%s/api/v1/results/pending?clinicId=%s", c.baseURL, clinicID), nil)
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

			var apiResp olympResultsResponse
			if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
				return fmt.Errorf("olymp: decode results: %w", err)
			}

			cID, _ := uuid.Parse(clinicID)
			results = make([]*domain.LabResult, 0, len(apiResp.Results))
			for _, r := range apiResp.Results {
				results = append(results, mapResult(cID, &r))
			}
			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("olymp.GetPendingResults: %w", err)
	}
	return results, nil
}

// AcknowledgeResult подтверждает получение результата в Олимп.
func (c *Client) AcknowledgeResult(ctx context.Context, externalID string) error {
	return c.cb.Execute(func() error {
		return retry.Do(ctx, retry.WithMaxRetries(3, retry.NewExponential(100*time.Millisecond)), func(ctx context.Context) error {
			callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			req, err := http.NewRequestWithContext(callCtx, http.MethodPost,
				fmt.Sprintf("%s/api/v1/results/%s/acknowledge", c.baseURL, externalID), nil)
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
			return nil
		})
	})
}

// ── Internal types ────────────────────────────────────────────────────────────

type olympResultsResponse struct {
	Results []olympResult `json:"results"`
}

type olympResult struct {
	ID         string         `json:"id"`
	PatientID  string         `json:"patientId"`
	TestName   string         `json:"testName"`
	Format     string         `json:"format"` // "pdf", "json", "xml"
	FileURL    string         `json:"fileUrl"`
	Data       map[string]any `json:"data"`
	ReceivedAt string         `json:"receivedAt"`
}

func mapResult(clinicID uuid.UUID, r *olympResult) *domain.LabResult {
	receivedAt, _ := time.Parse(time.RFC3339, r.ReceivedAt)
	patientID, _ := uuid.Parse(r.PatientID)
	return &domain.LabResult{
		ID:          uuid.New(),
		ClinicID:    clinicID,
		PatientID:   patientID,
		ExternalID:  r.ID,
		LabProvider: "olymp",
		TestName:    r.TestName,
		Format:      domain.LabResultFormat(r.Format),
		FileURL:     r.FileURL,
		Data:        r.Data,
		ReceivedAt:  receivedAt,
	}
}
