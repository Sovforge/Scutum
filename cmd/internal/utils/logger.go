package utils

import (
	"context"
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
}

var globalSink ObsSink

// SetObsSink registers a persistent sink that receives all entries.
// Call once at startup after the store is ready.
func SetObsSink(s ObsSink) { globalSink = s }

// ── In-memory ring buffers ────────────────────────────────────────────────────

const logRingSize   = 500
const auditRingSize = 1000
const traceRingSize = 500

type LogEntry struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

type AuditEntry struct {
	Time     time.Time         `json:"time"`
	Action   string            `json:"action"`
	Method   string            `json:"method"`
	Path     string            `json:"path"`
	TraceID  string            `json:"trace_id,omitempty"`
	ClientIP string            `json:"client_ip,omitempty"`
	Extra    map[string]string `json:"extra,omitempty"`
}

type TraceEntry struct {
	Time       time.Time `json:"time"`
	Name       string    `json:"name"`
	DurationMs int64     `json:"duration_ms"`
	Status     string    `json:"status"`
	Error      string    `json:"error,omitempty"`
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

func GetTraceEntries() []TraceEntry {
	traceRingMu.RLock()
	defer traceRingMu.RUnlock()
	out := make([]TraceEntry, len(traceRing))
	copy(out, traceRing)
	return out
}

// ─────────────────────────────────────────────────────────────────────────────

type Logger struct {
	logger  *slog.Logger
	auditer *slog.Logger
	mu      sync.RWMutex
}

var (
	DefaultLogger *Logger
	once           sync.Once
)

func InitLogger(level slog.Level, auditEnabled bool) *Logger {
	once.Do(func() {
		DefaultLogger = newLogger(level, auditEnabled)
	})
	return DefaultLogger
}

func newLogger(level slog.Level, auditEnabled bool) *Logger {
	opts := &slog.HandlerOptions{
		Level: level,
		AddSource: true,
	}

	console := slog.NewTextHandler(os.Stderr, opts)

	var auditHandler slog.Handler
	if auditEnabled {
		auditFile, err := os.OpenFile("/app/data/audit.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			auditHandler = console
		} else {
			auditHandler = slog.NewTextHandler(auditFile, opts)
		}
	} else {
		auditHandler = console
	}

	return &Logger{
		logger:  slog.New(console),
		auditer: slog.New(auditHandler),
	}
}

func (l *Logger) Debug(msg string, args ...any) {
	l.logger.Debug(msg, args...)
	appendLog(LogEntry{Time: time.Now(), Level: "debug", Message: msg})
}

func (l *Logger) Info(msg string, args ...any) {
	l.logger.Info(msg, args...)
	appendLog(LogEntry{Time: time.Now(), Level: "info", Message: msg})
}

func (l *Logger) Warn(msg string, args ...any) {
	l.logger.Warn(msg, args...)
	appendLog(LogEntry{Time: time.Now(), Level: "warn", Message: msg})
}

func (l *Logger) Error(msg string, args ...any) {
	l.logger.Error(msg, args...)
	appendLog(LogEntry{Time: time.Now(), Level: "error", Message: msg})
}

func (l *Logger) Fatal(msg string, args ...any) {
	l.logger.Error(msg, args...)
	os.Exit(1)
}

func (l *Logger) Audit(action string, args ...any) {
	l.auditer.Info(action, args...)
	entry := AuditEntry{
		Time:   time.Now(),
		Action: action,
		Extra:  make(map[string]string),
	}
	// Extract basic fields from args if present (best effort)
	for i := 0; i < len(args); i += 2 {
		if i+1 >= len(args) {
			break
		}
		key, ok := args[i].(string)
		if !ok {
			continue
		}
		val := fmt.Sprintf("%v", args[i+1])
		switch key {
		case "method":
			entry.Method = val
		case "path":
			entry.Path = val
		case "trace_id":
			entry.TraceID = val
		case "client_ip":
			entry.ClientIP = val
		default:
			entry.Extra[key] = val
		}
	}
	AppendAudit(entry)
}


func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		logger:  l.logger.With(args...),
		auditer: l.auditer.With(args...),
	}
}

type Trace struct {
	logger *Logger
	name   string
	start  time.Time
	ctx    context.Context
	err    error
}

func (l *Logger) Trace(ctx context.Context, name string) *Trace {
	return &Trace{
		logger: l,
		name:   name,
		start:  time.Now(),
		ctx:    ctx,
	}
}

func (t *Trace) End(err error) {
	t.err = err
	elapsed := time.Since(t.start)
	entry := TraceEntry{
		Time:       time.Now(),
		Name:       t.name,
		DurationMs: elapsed.Milliseconds(),
		Status:     "ok",
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