package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"scutum/cmd/internal/auth"
	"scutum/cmd/internal/utils"
)

type BaseHandler struct {
	Logger *utils.Logger
}

func NewBaseHandler(logger *utils.Logger) *BaseHandler {
	if logger == nil {
		logger = utils.DefaultLogger
		if logger == nil {
			logger = utils.InitLogger(slog.LevelInfo, false)
		}
	}
	return &BaseHandler{Logger: logger}
}

type ErrorResponse struct {
	Error   string    `json:"error"`
	Code    string    `json:"code,omitempty"`
	TraceID string    `json:"trace_id,omitempty"`
	Time    time.Time `json:"time"`
}

type SuccessResponse struct {
	Data      interface{} `json:"data,omitempty"`
	Message  string    `json:"message,omitempty"`
	TraceID  string    `json:"trace_id,omitempty"`
	Time     time.Time `json:"time"`
}

func (h *BaseHandler) WriteError(w http.ResponseWriter, r *http.Request, status int, err error) {
	trace, _ := utils.GetTrace(r.Context())

	resp := ErrorResponse{
		Error:   err.Error(),
		Time:   time.Now(),
		TraceID: trace.ID,
	}

	if status >= 500 {
		h.Logger.Error("request error",
			"error", err.Error(),
			"status", status,
			"method", r.Method,
			"path", r.URL.Path,
			"trace_id", trace.ID,
		)
	} else {
		h.Logger.Warn("request error",
			"error", err.Error(),
			"status", status,
			"method", r.Method,
			"path", r.URL.Path,
			"trace_id", trace.ID,
		)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

func (h *BaseHandler) WriteJSON(w http.ResponseWriter, r *http.Request, status int, data interface{}) {
	trace, _ := utils.GetTrace(r.Context())

	resp := SuccessResponse{
		Data:     data,
		Time:    time.Now(),
		TraceID: trace.ID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

func (h *BaseHandler) LogRequest(r *http.Request, status int, duration time.Duration) {
	trace, _ := utils.GetTrace(r.Context())

	h.Logger.Info("request completed",
		"method", r.Method,
		"path", r.URL.Path,
		"status", status,
		"duration_ms", duration.Milliseconds(),
		"trace_id", trace.ID,
	)
}

func (h *BaseHandler) Audit(action string, r *http.Request, args ...any) {
	trace, _ := utils.GetTrace(r.Context())

	actor, actorID := "", ""
	if claims, ok := auth.ClaimsFromContext(r.Context()); ok {
		actor = claims.Username
		actorID = claims.UserID
	}

	attrs := []any{
		"method",    r.Method,
		"path",      r.URL.Path,
		"trace_id",  trace.ID,
		"client_ip", r.RemoteAddr,
		"actor",     actor,
		"actor_id",  actorID,
	}
	attrs = append(attrs, args...)

	// Logger.Audit writes to the audit log file and calls AppendAudit once.
	// Do NOT call AppendAudit separately — that would double-record every event.
	h.Logger.Audit(action, attrs...)
}

type HandlerFunc func(w http.ResponseWriter, r *http.Request)

func (h *BaseHandler) Wrap(fn HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := utils.WithTrace(r.Context(), r.URL.Path)
		start := time.Now()
		duration := time.Duration(0)
		status := http.StatusOK

		defer func() {
			duration = time.Since(start)
			if recoverErr := recover(); recoverErr != nil {
				h.WriteError(w, r, http.StatusInternalServerError, utils.ErrInternal)
				h.LogRequest(r, http.StatusInternalServerError, duration)
			}
		}()

		fn(w, r.WithContext(ctx))

		status = getStatus(w)
		h.LogRequest(r, status, duration)
	}
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func getStatus(w http.ResponseWriter) int {
	if rw, ok := w.(*responseWriter); ok {
		return rw.status
	}
	return http.StatusOK
}