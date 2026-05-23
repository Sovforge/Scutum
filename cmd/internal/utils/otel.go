package utils

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// ── OTLP JSON format structures ───────────────────────────────────────────────

type otlpAnyValue struct {
	StringValue *string      `json:"stringValue"`
	IntValue    *json.Number `json:"intValue"`
	DoubleValue *float64     `json:"doubleValue"`
	BoolValue   *bool        `json:"boolValue"`
}

type otlpKV struct {
	Key   string       `json:"key"`
	Value otlpAnyValue `json:"value"`
}

func otlpAttrsToMap(attrs []otlpKV) map[string]string {
	if len(attrs) == 0 {
		return nil
	}
	m := make(map[string]string, len(attrs))
	for _, a := range attrs {
		switch {
		case a.Value.StringValue != nil:
			m[a.Key] = *a.Value.StringValue
		case a.Value.IntValue != nil:
			m[a.Key] = a.Value.IntValue.String()
		case a.Value.DoubleValue != nil:
			m[a.Key] = strconv.FormatFloat(*a.Value.DoubleValue, 'f', -1, 64)
		case a.Value.BoolValue != nil:
			if *a.Value.BoolValue {
				m[a.Key] = "true"
			} else {
				m[a.Key] = "false"
			}
		}
	}
	return m
}

func otlpServiceName(attrs []otlpKV) string {
	for _, a := range attrs {
		if a.Key == "service.name" && a.Value.StringValue != nil {
			return *a.Value.StringValue
		}
	}
	return ""
}

// ── Traces ────────────────────────────────────────────────────────────────────

type otlpTracesPayload struct {
	ResourceSpans []struct {
		Resource struct {
			Attributes []otlpKV `json:"attributes"`
		} `json:"resource"`
		ScopeSpans []struct {
			Spans []struct {
				TraceID           string       `json:"traceId"`
				SpanID            string       `json:"spanId"`
				ParentSpanID      string       `json:"parentSpanId"`
				Name              string       `json:"name"`
				Kind              int          `json:"kind"`
				StartTimeUnixNano json.Number  `json:"startTimeUnixNano"`
				EndTimeUnixNano   json.Number  `json:"endTimeUnixNano"`
				Status            struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				} `json:"status"`
				Attributes []otlpKV `json:"attributes"`
			} `json:"spans"`
		} `json:"scopeSpans"`
	} `json:"resourceSpans"`
}

var otlpKindNames = map[int]string{
	0: "unspecified", 1: "internal", 2: "server",
	3: "client", 4: "producer", 5: "consumer",
}

func ParseOTLPTraces(body []byte) ([]TraceEntry, error) {
	var payload otlpTracesPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	var out []TraceEntry
	for _, rs := range payload.ResourceSpans {
		service := otlpServiceName(rs.Resource.Attributes)
		for _, ss := range rs.ScopeSpans {
			for _, s := range ss.Spans {
				startNs, _ := s.StartTimeUnixNano.Int64()
				endNs, _ := s.EndTimeUnixNano.Int64()
				startTime := time.Unix(0, startNs)
				durationMs := int64(0)
				if endNs > startNs {
					durationMs = (endNs - startNs) / 1e6
				}
				status := "unset"
				switch s.Status.Code {
				case 1:
					status = "ok"
				case 2:
					status = "error"
				}
				kind := otlpKindNames[s.Kind]
				if kind == "" {
					kind = "internal"
				}
				attrs := otlpAttrsToMap(s.Attributes)
				entry := TraceEntry{
					TraceID:      s.TraceID,
					SpanID:       s.SpanID,
					ParentSpanID: s.ParentSpanID,
					Name:         s.Name,
					Service:      service,
					Kind:         kind,
					Time:         startTime,
					DurationMs:   durationMs,
					Status:       status,
					Error:        s.Status.Message,
					Source:       "otlp",
					Attributes:   attrs,
				}
				out = append(out, entry)
			}
		}
	}
	return out, nil
}

// ── Logs ──────────────────────────────────────────────────────────────────────

type otlpLogsPayload struct {
	ResourceLogs []struct {
		Resource struct {
			Attributes []otlpKV `json:"attributes"`
		} `json:"resource"`
		ScopeLogs []struct {
			LogRecords []struct {
				TimeUnixNano         json.Number  `json:"timeUnixNano"`
				ObservedTimeUnixNano json.Number  `json:"observedTimeUnixNano"`
				SeverityNumber       int          `json:"severityNumber"`
				SeverityText         string       `json:"severityText"`
				Body                 otlpAnyValue `json:"body"`
				TraceID              string       `json:"traceId"`
				SpanID               string       `json:"spanId"`
				Attributes           []otlpKV     `json:"attributes"`
			} `json:"logRecords"`
		} `json:"scopeLogs"`
	} `json:"resourceLogs"`
}

// severityToLevel maps OTEL severity numbers to log level strings.
func severityToLevel(n int) string {
	switch {
	case n >= 17:
		return "fatal"
	case n >= 13:
		return "error"
	case n >= 9:
		return "warn"
	case n >= 5:
		return "info"
	default:
		return "debug"
	}
}

func ParseOTLPLogs(body []byte) ([]LogEntry, error) {
	var payload otlpLogsPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	var out []LogEntry
	for _, rl := range payload.ResourceLogs {
		service := otlpServiceName(rl.Resource.Attributes)
		for _, sl := range rl.ScopeLogs {
			for _, rec := range sl.LogRecords {
				ns, _ := rec.TimeUnixNano.Int64()
				if ns == 0 {
					ns, _ = rec.ObservedTimeUnixNano.Int64()
				}
				t := time.Now()
				if ns > 0 {
					t = time.Unix(0, ns)
				}
				msg := ""
				if rec.Body.StringValue != nil {
					msg = *rec.Body.StringValue
				}
				level := rec.SeverityText
				if level == "" {
					level = severityToLevel(rec.SeverityNumber)
				}
				level = strings.ToLower(level)
				out = append(out, LogEntry{
					Time:       t,
					Level:      level,
					Message:    msg,
					Service:    service,
					Source:     "otlp",
					TraceID:    rec.TraceID,
					SpanID:     rec.SpanID,
					Attributes: otlpAttrsToMap(rec.Attributes),
				})
			}
		}
	}
	return out, nil
}

// ── Metrics ───────────────────────────────────────────────────────────────────

type otlpMetricsPayload struct {
	ResourceMetrics []struct {
		Resource struct {
			Attributes []otlpKV `json:"attributes"`
		} `json:"resource"`
		ScopeMetrics []struct {
			Metrics []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				Unit        string `json:"unit"`
				Gauge       *struct {
					DataPoints []otlpNumberDataPoint `json:"dataPoints"`
				} `json:"gauge"`
				Sum *struct {
					DataPoints []otlpNumberDataPoint `json:"dataPoints"`
				} `json:"sum"`
				Histogram *struct {
					DataPoints []otlpHistogramDataPoint `json:"dataPoints"`
				} `json:"histogram"`
			} `json:"metrics"`
		} `json:"scopeMetrics"`
	} `json:"resourceMetrics"`
}

type otlpNumberDataPoint struct {
	TimeUnixNano  json.Number  `json:"timeUnixNano"`
	AsDouble      *float64     `json:"asDouble"`
	AsInt         *json.Number `json:"asInt"`
	Attributes    []otlpKV     `json:"attributes"`
}

type otlpHistogramDataPoint struct {
	TimeUnixNano json.Number  `json:"timeUnixNano"`
	Count        json.Number  `json:"count"`
	Sum          *float64     `json:"sum"`
	Attributes   []otlpKV     `json:"attributes"`
}

func (dp otlpNumberDataPoint) floatValue() float64 {
	if dp.AsDouble != nil {
		return *dp.AsDouble
	}
	if dp.AsInt != nil {
		if v, err := dp.AsInt.Float64(); err == nil {
			return v
		}
	}
	return 0
}

func (dp otlpNumberDataPoint) timestamp() time.Time {
	ns, _ := dp.TimeUnixNano.Int64()
	if ns > 0 {
		return time.Unix(0, ns)
	}
	return time.Now()
}

func ParseOTLPMetrics(body []byte) ([]MetricPoint, error) {
	var payload otlpMetricsPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	var out []MetricPoint
	for _, rm := range payload.ResourceMetrics {
		service := otlpServiceName(rm.Resource.Attributes)
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Gauge != nil {
					for _, dp := range m.Gauge.DataPoints {
						out = append(out, MetricPoint{
							Time: dp.timestamp(), Name: m.Name, Service: service,
							Source: "otlp", Type: "gauge", Value: dp.floatValue(),
							Labels: otlpAttrsToMap(dp.Attributes),
						})
					}
				}
				if m.Sum != nil {
					for _, dp := range m.Sum.DataPoints {
						out = append(out, MetricPoint{
							Time: dp.timestamp(), Name: m.Name, Service: service,
							Source: "otlp", Type: "counter", Value: dp.floatValue(),
							Labels: otlpAttrsToMap(dp.Attributes),
						})
					}
				}
				if m.Histogram != nil {
					for _, dp := range m.Histogram.DataPoints {
						ns, _ := dp.TimeUnixNano.Int64()
						t := time.Now()
						if ns > 0 {
							t = time.Unix(0, ns)
						}
						count, _ := dp.Count.Float64()
						avg := 0.0
						if dp.Sum != nil && count > 0 {
							avg = *dp.Sum / count
						}
						out = append(out, MetricPoint{
							Time: t, Name: m.Name, Service: service,
							Source: "otlp", Type: "histogram", Value: avg,
							Labels: otlpAttrsToMap(dp.Attributes),
						})
					}
				}
			}
		}
	}
	return out, nil
}

// ── Log-line span extractor (for Docker/K8s log scraping) ────────────────────

// ParseSpanFromLogLine tries to extract an OTEL-compatible span from a single
// JSON log line. Returns nil when the line doesn't look like a span.
func ParseSpanFromLogLine(line, service, source string) *TraceEntry {
	line = strings.TrimSpace(line)
	if len(line) == 0 || line[0] != '{' {
		return nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		return nil
	}

	// Must have a name-like field and a duration-like field.
	name := stringField(m, "name", "operation", "span_name", "operationName", "op")
	if name == "" {
		return nil
	}
	durationMs := int64Field(m, "duration_ms", "duration", "elapsed_ms", "latency_ms", "elapsed", "latency", "took")
	if durationMs < 0 {
		durationMs = 0
	}

	t := timeField(m, "time", "timestamp", "ts", "@timestamp")
	if t.IsZero() {
		t = time.Now()
	}

	status := "ok"
	rawStatus := stringField(m, "status", "level", "lvl")
	if rawStatus == "error" || rawStatus == "err" || rawStatus == "ERROR" || rawStatus == "fatal" {
		status = "error"
	}

	errMsg := stringField(m, "error", "err", "exception")

	attrs := map[string]string{}
	for k, v := range m {
		switch k {
		case "name", "operation", "span_name", "operationName", "op",
			"duration_ms", "duration", "elapsed_ms", "latency_ms", "elapsed", "latency", "took",
			"time", "timestamp", "ts", "@timestamp",
			"status", "level", "lvl", "error", "err", "exception",
			"traceId", "trace_id", "spanId", "span_id", "parentSpanId", "parent_span_id":
			continue
		}
		attrs[k] = fmt.Sprintf("%v", v)
	}
	if len(attrs) == 0 {
		attrs = nil
	}

	return &TraceEntry{
		TraceID:      stringField(m, "traceId", "trace_id"),
		SpanID:       stringField(m, "spanId", "span_id"),
		ParentSpanID: stringField(m, "parentSpanId", "parent_span_id"),
		Name:         name,
		Service:      service,
		Kind:         stringField(m, "kind", "span_kind"),
		Time:         t,
		DurationMs:   durationMs,
		Status:       status,
		Error:        errMsg,
		Source:       source,
		Attributes:   attrs,
	}
}

// ParseLogFromLine tries to extract a structured log entry from a JSON line.
// Returns nil when the line isn't a structured log.
func ParseLogFromLine(line, service, source string) *LogEntry {
	line = strings.TrimSpace(line)
	if len(line) == 0 || line[0] != '{' {
		// Plain text line — wrap as info log
		return &LogEntry{
			Time:    time.Now(),
			Level:   "info",
			Message: line,
			Service: service,
			Source:  source,
		}
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		return &LogEntry{Time: time.Now(), Level: "info", Message: line, Service: service, Source: source}
	}

	msg := stringField(m, "message", "msg", "body", "log", "text")
	if msg == "" {
		msg = line
	}
	level := strings.ToLower(stringField(m, "level", "lvl", "severity", "severityText"))
	if level == "" {
		level = "info"
	}
	t := timeField(m, "time", "timestamp", "ts", "@timestamp")
	if t.IsZero() {
		t = time.Now()
	}

	return &LogEntry{
		Time:    t,
		Level:   level,
		Message: msg,
		Service: service,
		Source:  source,
		TraceID: stringField(m, "traceId", "trace_id"),
		SpanID:  stringField(m, "spanId", "span_id"),
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func stringField(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

func int64Field(m map[string]interface{}, keys ...string) int64 {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		switch n := v.(type) {
		case float64:
			return int64(math.Round(n))
		case json.Number:
			if i, err := n.Int64(); err == nil {
				return i
			}
		case string:
			if i, err := strconv.ParseInt(n, 10, 64); err == nil {
				return i
			}
		}
	}
	return -1
}

func timeField(m map[string]interface{}, keys ...string) time.Time {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		switch s := v.(type) {
		case string:
			for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05.999999999Z"} {
				if t, err := time.Parse(layout, s); err == nil {
					return t
				}
			}
		case float64:
			// Unix timestamp (seconds or milliseconds)
			if s > 1e12 {
				return time.UnixMilli(int64(s))
			}
			return time.Unix(int64(s), 0)
		}
	}
	return time.Time{}
}

// Prometheus scrape line parser — returns a MetricPoint per measurement line.
func ParsePrometheusLine(line, service, source string) *MetricPoint {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return nil
	}
	// Format: metric_name{labels} value [timestamp]
	// or: metric_name value [timestamp]
	var nameAndLabels, rest string
	if idx := strings.Index(line, " "); idx >= 0 {
		nameAndLabels = line[:idx]
		rest = strings.TrimSpace(line[idx+1:])
	} else {
		return nil
	}

	// Parse value (first token of rest)
	var valueStr string
	if idx := strings.Index(rest, " "); idx >= 0 {
		valueStr = rest[:idx]
	} else {
		valueStr = rest
	}
	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return nil
	}

	name := nameAndLabels
	labels := map[string]string{}
	if idx := strings.Index(nameAndLabels, "{"); idx >= 0 {
		name = nameAndLabels[:idx]
		labelStr := nameAndLabels[idx+1:]
		labelStr = strings.TrimSuffix(labelStr, "}")
		for _, pair := range strings.Split(labelStr, ",") {
			parts := strings.SplitN(pair, "=", 2)
			if len(parts) == 2 {
				k := strings.TrimSpace(parts[0])
				v := strings.Trim(strings.TrimSpace(parts[1]), `"`)
				labels[k] = v
			}
		}
	}
	if len(labels) == 0 {
		labels = nil
	}

	return &MetricPoint{
		Time:    time.Now(),
		Name:    name,
		Service: service,
		Source:  source,
		Type:    "gauge",
		Value:   value,
		Labels:  labels,
	}
}
