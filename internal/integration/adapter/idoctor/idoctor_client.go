package idoctor

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

// Client реализует adapter.AggregatorAdapter для iDoctor API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	cb         *adapter.CircuitBreaker
}

// New создаёт iDoctor клиент.
func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		cb:         adapter.NewCircuitBreaker(),
	}
}

// GetNewAppointments возвращает новые записи из iDoctor с момента since.
func (c *Client) GetNewAppointments(ctx context.Context, clinicID string, since time.Time) ([]*domain.ExternalAppointment, error) {
	var appointments []*domain.ExternalAppointment
	err := c.cb.Execute(func() error {
		return retry.Do(ctx, retry.WithMaxRetries(3, retry.NewExponential(100*time.Millisecond)), func(ctx context.Context) error {
			callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			url := fmt.Sprintf("%s/api/v2/clinics/%s/appointments?since=%s",
				c.baseURL, clinicID, since.UTC().Format(time.RFC3339))

			req, err := http.NewRequestWithContext(callCtx, http.MethodGet, url, nil)
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

			var apiResp iDoctorAppointmentsResponse
			if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
				return fmt.Errorf("idoctor: decode appointments: %w", err)
			}

			appointments = make([]*domain.ExternalAppointment, 0, len(apiResp.Appointments))
			for _, a := range apiResp.Appointments {
				appointments = append(appointments, mapAppointment(&a))
			}
			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("idoctor.GetNewAppointments: %w", err)
	}
	return appointments, nil
}

// UpdateAppointmentStatus обновляет статус записи в iDoctor.
func (c *Client) UpdateAppointmentStatus(ctx context.Context, externalID, status string) error {
	return c.cb.Execute(func() error {
		return retry.Do(ctx, retry.WithMaxRetries(3, retry.NewExponential(100*time.Millisecond)), func(ctx context.Context) error {
			callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			body, _ := json.Marshal(map[string]string{"status": status})
			req, err := http.NewRequestWithContext(callCtx, http.MethodPatch,
				fmt.Sprintf("%s/api/v2/appointments/%s/status", c.baseURL, externalID),
				bytesReader(body))
			if err != nil {
				return err
			}
			req.Header.Set("X-API-Key", c.apiKey)
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

// GetDoctorMapping возвращает маппинг внешних ID врачей на внутренние UUID.
func (c *Client) GetDoctorMapping(ctx context.Context, clinicID string) (map[string]uuid.UUID, error) {
	var mapping map[string]uuid.UUID
	err := c.cb.Execute(func() error {
		return retry.Do(ctx, retry.WithMaxRetries(3, retry.NewExponential(100*time.Millisecond)), func(ctx context.Context) error {
			callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			req, err := http.NewRequestWithContext(callCtx, http.MethodGet,
				fmt.Sprintf("%s/api/v2/clinics/%s/doctor-mapping", c.baseURL, clinicID), nil)
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

			var apiResp map[string]string // externalDoctorID -> internalUUID
			if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
				return fmt.Errorf("idoctor: decode doctor mapping: %w", err)
			}

			mapping = make(map[string]uuid.UUID, len(apiResp))
			for extID, intIDStr := range apiResp {
				id, err := uuid.Parse(intIDStr)
				if err != nil {
					continue
				}
				mapping[extID] = id
			}
			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("idoctor.GetDoctorMapping: %w", err)
	}
	return mapping, nil
}

// ── Internal types ────────────────────────────────────────────────────────────

type iDoctorAppointmentsResponse struct {
	Appointments []iDoctorAppointment `json:"appointments"`
}

type iDoctorAppointment struct {
	ID           string `json:"id"`
	DoctorID     string `json:"doctorId"`
	PatientName  string `json:"patientName"`
	PatientPhone string `json:"patientPhone"`
	PatientIIN   string `json:"patientIin"`
	ServiceName  string `json:"serviceName"`
	ScheduledAt  string `json:"scheduledAt"`
	Status       string `json:"status"`
	CreatedAt    string `json:"createdAt"`
}

func mapAppointment(a *iDoctorAppointment) *domain.ExternalAppointment {
	scheduledAt, _ := time.Parse(time.RFC3339, a.ScheduledAt)
	createdAt, _ := time.Parse(time.RFC3339, a.CreatedAt)
	return &domain.ExternalAppointment{
		ExternalID:     a.ID,
		ExternalSource: "idoctor",
		DoctorID:       a.DoctorID,
		PatientName:    a.PatientName,
		PatientPhone:   a.PatientPhone,
		PatientIIN:     a.PatientIIN,
		ServiceName:    a.ServiceName,
		ScheduledAt:    scheduledAt,
		Status:         a.Status,
		CreatedAt:      createdAt,
	}
}

// bytesReader — небольшой хелпер чтобы не импортировать bytes в каждом месте.
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
