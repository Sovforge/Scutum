package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// StartOTLPExporter starts a background goroutine that batches spans from the
// internal ring and POSTs them to endpoint every 5 seconds using OTLP/HTTP JSON.
// It returns immediately if endpoint is empty. The goroutine stops when ctx is done.
func StartOTLPExporter(ctx context.Context, endpoint string) {
	endpoint = strings.TrimRight(endpoint, "/")
	if endpoint == "" {
		return
	}
	tracesURL := endpoint + "/v1/traces"

	ch := make(chan TraceEntry, 2000)
	RegisterSpanExportChan(ch)

	client := &http.Client{Timeout: 10 * time.Second}

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		var batch []TraceEntry

		flush := func() {
			if len(batch) == 0 {
				return
			}
			if err := exportBatch(ctx, client, tracesURL, batch); err != nil {
				DefaultLogger.Warn("OTLP export failed", "error", err, "spans", len(batch))
			}
			batch = batch[:0]
		}

		for {
			select {
			case <-ctx.Done():
				flush()
				return
			case span := <-ch:
				batch = append(batch, span)
				if len(batch) >= 500 {
					flush()
				}
			case <-ticker.C:
				flush()
			}
		}
	}()
}

func exportBatch(ctx context.Context, client *http.Client, url string, spans []TraceEntry) error {
	payload := buildOTLPPayload(spans)
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("post: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("OTLP endpoint returned %d", resp.StatusCode)
	}
	return nil
}

// buildOTLPPayload converts a batch of TraceEntry values into the OTLP/HTTP JSON wire format.
func buildOTLPPayload(spans []TraceEntry) map[string]any {
	// Group by service
	byService := map[string][]TraceEntry{}
	for _, s := range spans {
		svc := s.Service
		if svc == "" {
			svc = "scutum"
		}
		byService[svc] = append(byService[svc], s)
	}

	var resourceSpans []any
	for svc, batch := range byService {
		var otlpSpans []any
		for _, s := range batch {
			startNs := s.Time.UnixNano()
			endNs := startNs + s.DurationMs*int64(time.Millisecond)

			statusCode := 1 // ok
			if s.Status == "error" {
				statusCode = 2
			}

			var attrs []any
			for k, v := range s.Attributes {
				attrs = append(attrs, map[string]any{
					"key":   k,
					"value": map[string]any{"stringValue": v},
				})
			}

			otlpSpans = append(otlpSpans, map[string]any{
				"traceId":           s.TraceID,
				"spanId":            s.SpanID,
				"parentSpanId":      s.ParentSpanID,
				"name":              s.Name,
				"kind":              otlpKindInt(s.Kind),
				"startTimeUnixNano": fmt.Sprintf("%d", startNs),
				"endTimeUnixNano":   fmt.Sprintf("%d", endNs),
				"status":            map[string]any{"code": statusCode, "message": s.Error},
				"attributes":        attrs,
			})
		}

		resourceSpans = append(resourceSpans, map[string]any{
			"resource": map[string]any{
				"attributes": []any{
					map[string]any{
						"key":   "service.name",
						"value": map[string]any{"stringValue": svc},
					},
				},
			},
			"scopeSpans": []any{
				map[string]any{"spans": otlpSpans},
			},
		})
	}

	return map[string]any{"resourceSpans": resourceSpans}
}

func otlpKindInt(kind string) int {
	switch kind {
	case "internal":
		return 1
	case "server":
		return 2
	case "client":
		return 3
	case "producer":
		return 4
	case "consumer":
		return 5
	default:
		return 0
	}
}
