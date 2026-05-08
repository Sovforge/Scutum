package utils

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

type traceKey struct{}

var (
	traceIDCounter uint64
	traceIDMu      sync.Mutex
)

func GenerateTraceID() string {
	traceIDMu.Lock()
	defer traceIDMu.Unlock()
	traceIDCounter++
	data := make([]byte, 8)
	data[0] = byte(traceIDCounter >> 56)
	data[1] = byte(traceIDCounter >> 48)
	data[2] = byte(traceIDCounter >> 40)
	data[3] = byte(traceIDCounter >> 32)
	data[4] = byte(traceIDCounter >> 24)
	data[5] = byte(traceIDCounter >> 16)
	data[6] = byte(traceIDCounter >> 8)
	data[7] = byte(traceIDCounter)
	return hex.EncodeToString(data)
}

type TraceInfo struct {
	ID        string
	Name      string
	StartTime time.Time
	ParentID string
}

func WithTrace(ctx context.Context, name string) context.Context {
	traceID := GenerateTraceID()
	var parentID string
	if parent, ok := ctx.Value(traceKey{}).(TraceInfo); ok {
		parentID = parent.ID
	}
	return context.WithValue(ctx, traceKey{}, TraceInfo{
		ID:        traceID,
		Name:      name,
		StartTime: time.Now(),
		ParentID: parentID,
	})
}

func GetTrace(ctx context.Context) (TraceInfo, bool) {
	trace, ok := ctx.Value(traceKey{}).(TraceInfo)
	return trace, ok
}

type Span struct {
	TraceID string
	Name    string
	Start   time.Time
	end     time.Time
	err     error
_attrs  []any
	mu      sync.Mutex
	Logger  *Logger
}

func StartSpan(ctx context.Context, logger *Logger, name string) *Span {
	trace, _ := GetTrace(ctx)
	return &Span{
		TraceID: trace.ID,
		Name:    name,
		Start:   time.Now(),
		Logger:  logger,
	}
}

func (s *Span) End(err error) {
	s.mu.Lock()
	s.end = time.Now()
	s.err = err
	s.mu.Unlock()

	elapsed := s.end.Sub(s.Start)

	attrs := []any{
		"trace_id", s.TraceID,
		"name", s.Name,
		"duration_ms", elapsed.Milliseconds(),
	}

	if s.err != nil {
		attrs = append(attrs, "error", s.err.Error())
		s.Logger.Error("span ended with error", attrs...)
	} else {
		s.Logger.Debug("span ended", attrs...)
	}
}

func (s *Span) SetAttribute(key, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s._attrs = append(s._attrs, key, value)
}

type ErrorHandler struct {
	logger *Logger
}

func (e *ErrorHandler) Handle(err error) error {
	if err == nil {
		return nil
	}
	e.logger.Error("error",
		"error", err.Error(),
		"timestamp", time.Now().Format(time.RFC3339),
	)
	return err
}

func NewErrorHandler(logger *Logger) *ErrorHandler {
	return &ErrorHandler{logger: logger}
}

type Error struct {
	Code    string
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error {
	return e.Cause
}

func NewError(code, message string, cause error) *Error {
	return &Error{Code: code, Message: message, Cause: cause}
}

func WrapError(err error, code, message string) *Error {
	if err == nil {
		return nil
	}
	return &Error{Code: code, Message: message, Cause: err}
}

var (
	ErrNotFound      = NewError("NOT_FOUND", "resource not found", nil)
	ErrUnauthorized = NewError("UNAUTHORIZED", "authentication required", nil)
	ErrForbidden    = NewError("FORBIDDEN", "permission denied", nil)
	ErrInternal    = NewError("INTERNAL", "internal server error", nil)
	ErrBadRequest  = NewError("BAD_REQUEST", "invalid request", nil)
)