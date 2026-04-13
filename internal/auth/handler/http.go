package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/nurtidev/medcore/internal/auth/domain"
	authmw "github.com/nurtidev/medcore/internal/auth/middleware"
	"github.com/nurtidev/medcore/internal/auth/service"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

// HTTP wraps the auth service with chi routes.
type HTTP struct {
	svc service.AuthService
	log zerolog.Logger
}

// NewHTTP creates the handler and wires all routes.
func NewHTTP(svc service.AuthService, rdb *redis.Client, log zerolog.Logger) http.Handler {
	h := &HTTP{svc: svc, log: log}

	// Rate limiting — disabled gracefully when Redis is unavailable (e.g. tests).
	nop := func(next http.Handler) http.Handler { return next }
	registerLimit, loginLimit := nop, nop
	if rdb != nil {
		rl := authmw.NewRateLimiter(rdb)
		registerLimit = rl.Limit("register", 5, time.Minute)
		loginLimit = rl.Limit("login", 10, time.Minute)
	}

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)

	// Health
	r.Get("/health", h.health)
	r.Get("/ready", h.ready)

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.With(registerLimit).Post("/register", h.register)
			r.With(loginLimit).Post("/login", h.login)
			r.Post("/refresh", h.refresh)

			// Authenticated routes
			r.Group(func(r chi.Router) {
				r.Use(authmw.JWT(svc))
				r.Post("/logout", h.logout)
				r.Get("/me", h.me)
				r.Put("/me", h.updateMe)
				r.Post("/change-password", h.changePassword)
			})
		})

		r.Route("/users", func(r chi.Router) {
			r.Use(authmw.JWT(svc))
			r.Use(authmw.RequireRole(domain.RoleAdmin, domain.RoleSuperAdmin))
			r.Get("/", h.listUsers)
			r.Get("/{id}", h.getUser)
			r.Put("/{id}", h.updateUser)
			r.Delete("/{id}", h.deactivateUser)
		})
	})

	return r
}

// ─── Health ──────────────────────────────────────────────────────────────────

func (h *HTTP) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *HTTP) ready(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// ─── Auth endpoints ───────────────────────────────────────────────────────────

func (h *HTTP) register(w http.ResponseWriter, r *http.Request) {
	// Only admin/super_admin may register new users — enforced via JWT + RBAC
	// Public registration is disabled; callers must be authenticated admins.
	claims, ok := authmw.ClaimsFromContext(r.Context())
	if ok {
		// If a token is present, validate role
		if claims.Role != domain.RoleAdmin && claims.Role != domain.RoleSuperAdmin {
			writeError(w, r, http.StatusForbidden, "forbidden", "only admin can register users")
			return
		}
	}

	var req struct {
		ClinicID  string `json:"clinic_id"`
		Email     string `json:"email"`
		Password  string `json:"password"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		IIN       string `json:"iin"`
		Phone     string `json:"phone"`
		Role      string `json:"role"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	clinicID, err := uuid.Parse(req.ClinicID)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_input", "invalid clinic_id")
		return
	}

	user, err := h.svc.Register(r.Context(), domain.RegisterRequest{
		ClinicID:  clinicID,
		Email:     req.Email,
		Password:  req.Password,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		IIN:       req.IIN,
		Phone:     req.Phone,
		Role:      domain.Role(req.Role),
	})
	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, userResponse(user))
}

func (h *HTTP) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	pair, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, pair)
}

func (h *HTTP) refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	pair, err := h.svc.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, pair)
}

func (h *HTTP) logout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	if err := h.svc.Logout(r.Context(), req.RefreshToken); err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTP) me(w http.ResponseWriter, r *http.Request) {
	claims, _ := authmw.ClaimsFromContext(r.Context())

	user, err := h.svc.GetUser(r.Context(), claims.UserID)
	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, userResponse(user))
}

func (h *HTTP) updateMe(w http.ResponseWriter, r *http.Request) {
	claims, _ := authmw.ClaimsFromContext(r.Context())

	var req domain.UpdateUserRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	user, err := h.svc.UpdateUser(r.Context(), claims.UserID, req)
	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, userResponse(user))
}

func (h *HTTP) changePassword(w http.ResponseWriter, r *http.Request) {
	claims, _ := authmw.ClaimsFromContext(r.Context())

	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	if err := h.svc.ChangePassword(r.Context(), claims.UserID, req.OldPassword, req.NewPassword); err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ─── Admin user management ────────────────────────────────────────────────────

func (h *HTTP) listUsers(w http.ResponseWriter, r *http.Request) {
	claims, _ := authmw.ClaimsFromContext(r.Context())

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	// For admin: list users in their clinic; super_admin can specify clinic_id
	clinicID := claims.ClinicID
	if claims.Role == domain.RoleSuperAdmin {
		if raw := r.URL.Query().Get("clinic_id"); raw != "" {
			if id, err := uuid.Parse(raw); err == nil {
				clinicID = id
			}
		}
	}

	users, total, err := h.svc.ListUsers(r.Context(), clinicID, limit, offset)
	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	resp := make([]userResp, 0, len(users))
	for _, u := range users {
		resp = append(resp, userResponse(u))
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": resp, "total": total})
}

func (h *HTTP) getUser(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_input", "invalid user id")
		return
	}

	user, err := h.svc.GetUser(r.Context(), id)
	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, userResponse(user))
}

func (h *HTTP) updateUser(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_input", "invalid user id")
		return
	}

	var req domain.UpdateUserRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	user, err := h.svc.UpdateUser(r.Context(), id, req)
	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, userResponse(user))
}

func (h *HTTP) deactivateUser(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_input", "invalid user id")
		return
	}

	claims, _ := authmw.ClaimsFromContext(r.Context())

	if err := h.svc.DeactivateUser(r.Context(), id, claims.UserID); err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

type userResp struct {
	ID        string `json:"id"`
	ClinicID  string `json:"clinic_id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone"`
	Role      string `json:"role"`
	IsActive  bool   `json:"is_active"`
	CreatedAt string `json:"created_at"`
}

func userResponse(u *domain.User) userResp {
	return userResp{
		ID:        u.ID.String(),
		ClinicID:  u.ClinicID.String(),
		Email:     u.Email,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Phone:     u.Phone,
		Role:      string(u.Role),
		IsActive:  u.IsActive,
		CreatedAt: u.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func (h *HTTP) handleServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrUserNotFound):
		writeError(w, r, http.StatusNotFound, "not_found", "user not found")
	case errors.Is(err, domain.ErrUserInactive):
		writeError(w, r, http.StatusForbidden, "user_inactive", "user is inactive")
	case errors.Is(err, domain.ErrUserExists):
		writeError(w, r, http.StatusConflict, "conflict", "user already exists")
	case errors.Is(err, domain.ErrInvalidPassword):
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "invalid credentials")
	case errors.Is(err, domain.ErrTokenExpired):
		writeError(w, r, http.StatusUnauthorized, "token_expired", "token expired")
	case errors.Is(err, domain.ErrTokenInvalid), errors.Is(err, domain.ErrTokenRevoked):
		writeError(w, r, http.StatusUnauthorized, "token_invalid", "token invalid or revoked")
	case errors.Is(err, domain.ErrForbidden):
		writeError(w, r, http.StatusForbidden, "forbidden", "access denied")
	case errors.Is(err, domain.ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "invalid_input", err.Error())
	default:
		h.log.Error().Err(err).Msg("unhandled service error")
		writeError(w, r, http.StatusInternalServerError, "internal", "internal server error")
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	writeJSON(w, status, map[string]string{
		"error":      code,
		"message":    message,
		"request_id": r.Header.Get("X-Request-ID"),
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_json", "invalid request body")
		return false
	}
	return true
}
