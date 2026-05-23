package utils

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"log/slog"
	"os"
	"sync"
	"time"
)

// ObsSink receives log/audit/trace entries for persistent storage (e.g. a database).
// Implementations must be non-blocking; use goroutines internally if needed.
type ObsSink interface {
	PersistAudit(e AuditEntry)
	PersistLog(e LogEntry)
	PersistTrace(e TraceEntry)
	PersistMetric(e MetricPoint)
}

// EventOutcome values for AuditEntry.Outcome.
const (
	OutcomeSuccess = "success"
	OutcomeFailure = "failure"
)

var globalSink ObsSink

// SetObsSink registers a persistent sink that receives all entries.
// Call once at startup after the store is ready.
func SetObsSink(s ObsSink) { globalSink = s }

// ── Context keys for trace propagation ───────────────────────────────────────

type traceIDKey struct{}
type spanIDKey struct{}

func WithTraceContext(ctx context.Context, traceID, spanID string) context.Context {
	ctx = context.WithValue(ctx, traceIDKey{}, traceID)
	return context.WithValue(ctx, spanIDKey{}, spanID)
}

func TraceIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(traceIDKey{}).(string); ok {
		return v
	}
	return ""
}

func SpanIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(spanIDKey{}).(string); ok {
		return v
	}
	return ""
}

// ── ID generation (OTEL standard: 16-byte trace, 8-byte span, hex-encoded) ───

func NewTraceID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func NewSpanID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// ── In-memory ring buffers ────────────────────────────────────────────────────

const logRingSize   = 500
const auditRingSize = 1000
const traceRingSize = 1000

type LogEntry struct {
	Time       time.Time         `json:"time"`
	Level      string            `json:"level"`
	Message    string            `json:"message"`
	Service    string            `json:"service,omitempty"`
	Source     string            `json:"source,omitempty"` // internal|otlp|docker|k8s
	TraceID    string            `json:"trace_id,omitempty"`
	SpanID     string            `json:"span_id,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

type AuditEntry struct {
	Time     time.Time         `json:"time"`
	Action   string            `json:"action"`
	Actor    string            `json:"actor,omitempty"`
	ActorID  string            `json:"actor_id,omitempty"`
	Outcome  string            `json:"outcome,omitempty"`
	Method   string            `json:"method"`
	Path     string            `json:"path"`
	TraceID  string            `json:"trace_id,omitempty"`
	ClientIP string            `json:"client_ip,omitempty"`
	Extra    map[string]string `json:"extra,omitempty"`
}

// TraceEntry is an OpenTelemetry-compatible span record.
type TraceEntry struct {
	// OTEL identifiers
	TraceID      string `json:"trace_id,omitempty"`
	SpanID       string `json:"span_id,omitempty"`
	ParentSpanID string `json:"parent_span_id,omitempty"`
	// Core fields
	Name    string    `json:"name"`
	Service string    `json:"service,omitempty"`
	Kind    string    `json:"kind,omitempty"` // server|client|internal|producer|consumer
	Time    time.Time `json:"time"`
	// Timing
	DurationMs int64 `json:"duration_ms"`
	// Status (OTEL: ok|error|unset)
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
	// Classification
	Source     string            `json:"source,omitempty"` // internal|otlp|docker|k8s
	Attributes map[string]string `json:"attributes,omitempty"`
}

// MetricPoint is a single OTEL-compatible metric data point.
type MetricPoint struct {
	Time    time.Time         `json:"time"`
	Name    string            `json:"name"`
	Service string            `json:"service,omitempty"`
	Source  string            `json:"source,omitempty"` // internal|otlp|docker|k8s
	Type    string            `json:"type"`             // gauge|counter|histogram|summary
	Value   float64           `json:"value"`
	Labels  map[string]string `json:"labels,omitempty"`
}

var (
	logRing     []LogEntry
	auditRing   []AuditEntry
	traceRing   []TraceEntry
	logRingMu   sync.RWMutex
	auditRingMu sync.RWMutex
	traceRingMu sync.RWMutex
)

func appendLog(e LogEntry) {
	logRingMu.Lock()
	defer logRingMu.Unlock()
	if len(logRing) >= logRingSize {
		logRing = logRing[1:]
	}
	logRing = append(logRing, e)
	if globalSink != nil {
		globalSink.PersistLog(e)
	}
}

func AppendAudit(e AuditEntry) {
	auditRingMu.Lock()
	defer auditRingMu.Unlock()
	if len(auditRing) >= auditRingSize {
		auditRing = auditRing[1:]
	}
	auditRing = append(auditRing, e)
	if globalSink != nil {
		globalSink.PersistAudit(e)
	}
}

func appendTrace(e TraceEntry) {
	traceRingMu.Lock()
	defer traceRingMu.Unlock()
	if len(traceRing) >= traceRingSize {
		traceRing = traceRing[1:]
	}
	traceRing = append(traceRing, e)
	if globalSink != nil {
		globalSink.PersistTrace(e)
	}
}

// AppendSpan is the public entrypoint for writing pre-built spans (e.g. from
// the HTTP middleware or OTLP ingest handler).
func AppendSpan(e TraceEntry) { appendTrace(e) }

// AppendMetric persists a metric data point to the ring and DB sink.
func AppendMetric(e MetricPoint) {
	if globalSink != nil {
		globalSink.PersistMetric(e)
	}
}

func GetLogEntries() []LogEntry {
	logRingMu.RLock()
	defer logRingMu.RUnlock()
	out := make([]LogEntry, len(logRing))
	copy(out, logRing)
	return out
}

func GetAuditEntries() []AuditEntry {
	auditRingMu.RLock()
	defer auditRingMu.RUnlock()
	out := make([]AuditEntry, len(auditRing))
	copy(out, auditRing)
	return out
}

func GetTraceEntries() []TraceEntry {
	traceRingMu.RLock()
	defer traceRingMu.RUnlock()
	out := make([]TraceEntry, len(traceRing))
	copy(out, traceRing)
	return out
}

// AppendExternalLog stores a log entry received from an external source
// (OTLP, container scrape, etc.) without writing to the slog handler.
func AppendExternalLog(e LogEntry) {
	appendLog(e)
}

// ─────────────────────────────────────────────────────────────────────────────

type Logger struct {
	logger  *slog.Logger
	auditer *slog.Logger
	level   *slog.LevelVar
	mu      sync.RWMutex
}

var (
	DefaultLogger *Logger
	once          sync.Once
)

func InitLogger(level slog.Level, auditEnabled bool) *Logger {
	once.Do(func() {
		DefaultLogger = newLogger(level, auditEnabled)
	})
	return DefaultLogger
}

func newLogger(level slog.Level, auditEnabled bool) *Logger {
	lv := &slog.LevelVar{}
	lv.Set(level)
	opts := &slog.HandlerOptions{
		Level:     lv,
		AddSource: true,
	}

	console := slog.NewJSONHandler(os.Stderr, opts)

	var auditHandler slog.Handler
	if auditEnabled {
		auditFile, err := os.OpenFile("/app/data/audit.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			auditHandler = console
		} else {
			auditHandler = slog.NewJSONHandler(auditFile, opts)
		}
	} else {
		auditHandler = console
	}

	return &Logger{
		logger:  slog.New(console),
		auditer: slog.New(auditHandler),
		level:   lv,
	}
}

func (l *Logger) Debug(msg string, args ...any) {
	l.logger.Debug(msg, args...)
	appendLog(LogEntry{Time: time.Now(), Level: "debug", Message: msg, Source: "internal", Service: "scutum"})
}

func (l *Logger) Info(msg string, args ...any) {
	l.logger.Info(msg, args...)
	appendLog(LogEntry{Time: time.Now(), Level: "info", Message: msg, Source: "internal", Service: "scutum"})
}

func (l *Logger) Warn(msg string, args ...any) {
	l.logger.Warn(msg, args...)
	appendLog(LogEntry{Time: time.Now(), Level: "warn", Message: msg, Source: "internal", Service: "scutum"})
}

func (l *Logger) Error(msg string, args ...any) {
	l.logger.Error(msg, args...)
	appendLog(LogEntry{Time: time.Now(), Level: "error", Message: msg, Source: "internal", Service: "scutum"})
}

func (l *Logger) Fatal(msg string, args ...any) {
	l.logger.Error(msg, args...)
	os.Exit(1)
}

func (l *Logger) Audit(action string, args ...any) {
	l.auditer.Info(action, args...)
	entry := AuditEntry{
		Time:    time.Now().UTC(),
		Action:  action,
		Outcome: OutcomeSuccess,
		Extra:   make(map[string]string),
	}
	for i := 0; i+1 < len(args); i += 2 {
		key, ok := args[i].(string)
		if !ok {
			continue
		}
		val := fmt.Sprintf("%v", args[i+1])
		switch key {
		case "method":    entry.Method = val
		case "path":      entry.Path = val
		case "trace_id":  entry.TraceID = val
		case "client_ip": entry.ClientIP = val
		case "actor":     entry.Actor = val
		case "actor_id":  entry.ActorID = val
		case "outcome":   entry.Outcome = val
		default:          entry.Extra[key] = val
		}
	}
	AppendAudit(entry)
}

func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		logger:  l.logger.With(args...),
		auditer: l.auditer.With(args...),
		level:   l.level,
	}
}

// Trace starts a new OTEL-compatible span. If ctx carries a parent trace ID
// it is used; otherwise a new trace ID is generated.
type Trace struct {
	logger       *Logger
	name         string
	service      string
	kind         string
	start        time.Time
	ctx          context.Context
	err          error
	traceID      string
	spanID       string
	parentSpanID string
	attrs        map[string]string
}

func (l *Logger) Trace(ctx context.Context, name string) *Trace {
	traceID := TraceIDFromContext(ctx)
	parentSpanID := SpanIDFromContext(ctx)
	if traceID == "" {
		traceID = NewTraceID()
	}
	return &Trace{
		logger:       l,
		name:         name,
		service:      "scutum",
		kind:         "internal",
		start:        time.Now(),
		ctx:          ctx,
		traceID:      traceID,
		spanID:       NewSpanID(),
		parentSpanID: parentSpanID,
	}
}

// WithAttrs adds key-value attributes to this span.
func (t *Trace) WithAttrs(attrs map[string]string) *Trace {
	t.attrs = attrs
	return t
}

func (t *Trace) End(err error) {
	t.err = err
	elapsed := time.Since(t.start)
	entry := TraceEntry{
		TraceID:      t.traceID,
		SpanID:       t.spanID,
		ParentSpanID: t.parentSpanID,
		Name:         t.name,
		Service:      t.service,
		Kind:         t.kind,
		Time:         t.start,
		DurationMs:   elapsed.Milliseconds(),
		Status:       "ok",
		Source:       "internal",
		Attributes:   t.attrs,
	}
	if err != nil {
		entry.Status = "error"
		entry.Error = err.Error()
		t.logger.Error("trace",
			"name", t.name,
			"duration_ms", elapsed.Milliseconds(),
			"error", err,
		)
	} else {
		t.logger.Debug("trace",
			"name", t.name,
			"duration_ms", elapsed.Milliseconds(),
		)
	}
	appendTrace(entry)
}

func (l *Logger) SetLevel(level slog.Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.level != nil {
		l.level.Set(level)
	}
}

type LogWriter struct {
	buf []byte
	mu  sync.Mutex
}

func (w *LogWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf = append(w.buf, p...)
	return len(p), nil
}

func (w *LogWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return string(w.buf)
}

func (w *LogWriter) Reset() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf = w.buf[:0]
}

func StdLogger(l *Logger) *log.Logger {
	writer := &LogWriter{}
	logger := log.New(writer, "", 0)
	return logger
}
