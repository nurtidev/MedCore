package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/nurtidev/medcore/internal/integration/domain"
	"github.com/nurtidev/medcore/internal/integration/service"
	"github.com/rs/zerolog"
)

// WebhookConfig содержит секреты для проверки подписей.
type WebhookConfig struct {
	IDoctorSecret string
	OlympSecret   string
	InvivoSecret  string
}

// Webhook — хендлер входящих webhooks от агрегаторов и лабораторий.
type Webhook struct {
	svc    service.IntegrationService
	cfg    WebhookConfig
	log    zerolog.Logger
}

// NewWebhook создаёт Webhook хендлер и регистрирует маршруты.
func NewWebhook(svc service.IntegrationService, cfg WebhookConfig, log zerolog.Logger) http.Handler {
	wh := &Webhook{svc: svc, cfg: cfg, log: log}

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)

	r.Post("/webhooks/idoctor", wh.iDoctor)
	r.Post("/webhooks/olymp", wh.olymp)
	r.Post("/webhooks/invivo", wh.invivo)

	return r
}

// iDoctor обрабатывает входящий webhook от iDoctor.
// Проверка HMAC-SHA256 подписи (X-IDoctor-Signature заголовок).
func (wh *Webhook) iDoctor(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		wh.log.Error().Err(err).Msg("idoctor webhook: read body")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	sig := r.Header.Get("X-IDoctor-Signature")
	if !verifyHMAC(body, sig, wh.cfg.IDoctorSecret) {
		wh.log.Warn().Msg("idoctor webhook: invalid signature")
		writeError(w, r, http.StatusUnauthorized, "invalid_signature", "invalid webhook signature")
		return
	}

	// Немедленно отвечаем 200 OK — обработка асинхронная
	w.WriteHeader(http.StatusOK)

	payload := domain.WebhookPayload{
		Provider:  "idoctor",
		EventType: r.Header.Get("X-IDoctor-Event"),
		Raw:       body,
	}
	if err := wh.svc.HandleIncomingAppointment(r.Context(), payload); err != nil {
		wh.log.Error().Err(err).Msg("idoctor webhook: handle")
	}
}

// olymp обрабатывает входящий webhook от лаборатории Олимп.
func (wh *Webhook) olymp(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		wh.log.Error().Err(err).Msg("olymp webhook: read body")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	sig := r.Header.Get("X-Olymp-Signature")
	if !verifyHMAC(body, sig, wh.cfg.OlympSecret) {
		wh.log.Warn().Msg("olymp webhook: invalid signature")
		writeError(w, r, http.StatusUnauthorized, "invalid_signature", "invalid webhook signature")
		return
	}

	// Немедленно 200 OK
	w.WriteHeader(http.StatusOK)

	if err := wh.svc.HandleLabWebhook(r.Context(), "olymp", body); err != nil {
		wh.log.Error().Err(err).Msg("olymp webhook: handle")
	}
}

// invivo обрабатывает входящий webhook от лаборатории Инвиво.
func (wh *Webhook) invivo(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		wh.log.Error().Err(err).Msg("invivo webhook: read body")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	sig := r.Header.Get("X-Invivo-Signature")
	if !verifyHMAC(body, sig, wh.cfg.InvivoSecret) {
		wh.log.Warn().Msg("invivo webhook: invalid signature")
		writeError(w, r, http.StatusUnauthorized, "invalid_signature", "invalid webhook signature")
		return
	}

	// Немедленно 200 OK
	w.WriteHeader(http.StatusOK)

	if err := wh.svc.HandleLabWebhook(r.Context(), "invivo", body); err != nil {
		wh.log.Error().Err(err).Msg("invivo webhook: handle")
	}
}

// verifyHMAC проверяет HMAC-SHA256 подпись.
func verifyHMAC(body []byte, signature, secret string) bool {
	if secret == "" {
		// в dev-режиме пропускаем проверку
		return true
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}
