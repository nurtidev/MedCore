package invivo

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

// Client реализует adapter.LaboratoryAdapter для лаборатории Инвиво.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	cb         *adapter.CircuitBreaker
}

// New создаёт клиент лаборатории Инвиво.
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
				fmt.Sprintf("%s/partner/results?clinic_id=%s&status=pending", c.baseURL, clinicID), nil)
			if err != nil {
				return err
			}
			req.Header.Set("Authorization", "ApiKey "+c.apiKey)

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

			var apiResp invivoResultsResponse
			if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
				return fmt.Errorf("invivo: decode results: %w", err)
			}

			cID, _ := uuid.Parse(clinicID)
			results = make([]*domain.LabResult, 0, len(apiResp.Items))
			for _, item := range apiResp.Items {
				results = append(results, mapResult(cID, &item))
			}
			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("invivo.GetPendingResults: %w", err)
	}
	return results, nil
}

// AcknowledgeResult подтверждает получение результата в Инвиво.
func (c *Client) AcknowledgeResult(ctx context.Context, externalID string) error {
	return c.cb.Execute(func() error {
		return retry.Do(ctx, retry.WithMaxRetries(3, retry.NewExponential(100*time.Millisecond)), func(ctx context.Context) error {
			callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			body, _ := json.Marshal(map[string]string{"resultId": externalID, "status": "received"})
			req, err := http.NewRequestWithContext(callCtx, http.MethodPost,
				fmt.Sprintf("%s/partner/results/confirm", c.baseURL),
				bytesReader(body))
			if err != nil {
				return err
			}
			req.Header.Set("Authorization", "ApiKey "+c.apiKey)
			req.Header.Set("Content-Type", "application/json")

			resp, err := c.httpClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode >= 400 {
				b, _ := io.ReadAll(resp.Body)
				httpErr := &adapter.HTTPError{StatusCode: resp.StatusCode, Body: string(b)}
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

type invivoResultsResponse struct {
	Items []invivoResult `json:"items"`
}

type invivoResult struct {
	ResultID   string         `json:"resultId"`
	PatientID  string         `json:"patientId"`
	TestName   string         `json:"testName"`
	OutputType string         `json:"outputType"` // "PDF", "JSON"
	FileLink   string         `json:"fileLink"`
	Payload    map[string]any `json:"payload"`
	IssuedAt   string         `json:"issuedAt"`
}

func mapResult(clinicID uuid.UUID, r *invivoResult) *domain.LabResult {
	receivedAt, _ := time.Parse(time.RFC3339, r.IssuedAt)
	patientID, _ := uuid.Parse(r.PatientID)

	format := domain.FormatJSON
	if r.OutputType == "PDF" {
		format = domain.FormatPDF
	}

	return &domain.LabResult{
		ID:          uuid.New(),
		ClinicID:    clinicID,
		PatientID:   patientID,
		ExternalID:  r.ResultID,
		LabProvider: "invivo",
		TestName:    r.TestName,
		Format:      format,
		FileURL:     r.FileLink,
		Data:        r.Payload,
		ReceivedAt:  receivedAt,
	}
}

type bytesReaderType struct {
	data []byte
	pos  int
}

func bytesReader(b []byte) *bytesReaderType {
	return &bytesReaderType{data: b}
}

func (r *bytesReaderType) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
