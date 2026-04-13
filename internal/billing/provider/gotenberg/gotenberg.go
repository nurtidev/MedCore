// Package gotenberg wraps the Gotenberg API for HTML→PDF conversion.
// Gotenberg docs: https://gotenberg.dev/docs/routes#convert-with-chromium
package gotenberg

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

// Client converts HTML to PDF via the Gotenberg service.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New creates a Gotenberg client.
// baseURL is e.g. "http://gotenberg:3000"
func New(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// HTMLtoPDF converts an HTML string to a PDF byte slice.
func (c *Client) HTMLtoPDF(ctx context.Context, html string) ([]byte, error) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)

	// Gotenberg expects the HTML as a file named "index.html" in a multipart form.
	part, err := mw.CreateFormFile("files", "index.html")
	if err != nil {
		return nil, fmt.Errorf("gotenberg: create form file: %w", err)
	}
	if _, err := io.WriteString(part, html); err != nil {
		return nil, fmt.Errorf("gotenberg: write html: %w", err)
	}
	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf("gotenberg: close multipart: %w", err)
	}

	url := c.baseURL + "/forms/chromium/convert/html"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return nil, fmt.Errorf("gotenberg: new request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gotenberg: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gotenberg: status %d: %s", resp.StatusCode, string(body))
	}

	pdf, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gotenberg: read response: %w", err)
	}

	return pdf, nil
}
