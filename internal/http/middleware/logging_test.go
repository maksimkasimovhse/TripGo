package middleware_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/maksimkasimovhse/TripGo/internal/http/middleware"
)

func TestRequestLogging(t *testing.T) {
	t.Parallel()

	for _, status := range []int{0, 200, 204, 400, 404, 409, 500, 503} {
		t.Run(fmt.Sprintf("status=%d", status), func(t *testing.T) {
			t.Parallel()

			var output bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&output, nil)).With("service", "trip-service")
			router := chi.NewRouter()
			router.Use(middleware.Recover(logger), middleware.RequestLogging(logger))
			router.Post("/trips/{tripId}", func(w http.ResponseWriter, r *http.Request) {
				if status != 0 {
					w.WriteHeader(status)
				}
				if status != http.StatusNoContent {
					if _, err := w.Write([]byte("private response body")); err != nil {
						t.Fatal(err)
					}
				}
			})

			req := httptest.NewRequest(http.MethodPost, "/trips/private-trip-id?token=private-query", strings.NewReader("private request body"))
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			wantStatus := status
			if wantStatus == 0 {
				wantStatus = http.StatusOK
			}
			records := decodeLogs(t, &output)
			if len(records) != 1 {
				t.Fatalf("records = %d, want 1: %+v", len(records), records)
			}
			record := records[0]
			wantLevel := "INFO"
			if wantStatus >= 500 {
				wantLevel = "ERROR"
			}
			if record["msg"] != "request handled" || record["level"] != wantLevel || record["status"] != float64(wantStatus) {
				t.Errorf("unexpected request record: %+v", record)
			}
			if record["route"] != "/trips/{tripId}" || record["method"] != "POST" || record["service"] != "trip-service" {
				t.Errorf("unexpected request attributes: %+v", record)
			}
			if duration, ok := record["duration_ms"].(float64); !ok || duration < 0 {
				t.Errorf("duration_ms = %v", record["duration_ms"])
			}
			for _, private := range []string{"private-trip-id", "private-query", "private request body", "private response body"} {
				if strings.Contains(output.String(), private) {
					t.Errorf("log contains %q", private)
				}
			}
			if rec.Code != wantStatus {
				t.Errorf("response status = %d, want %d", rec.Code, wantStatus)
			}
		})
	}
}

func TestRequestLoggingUnknownRoute(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	router := chi.NewRouter()
	router.Use(middleware.Recover(logger), middleware.RequestLogging(logger))
	router.Get("/known", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/private-unmatched-path", nil))

	records := decodeLogs(t, &output)
	if len(records) != 1 || records[0]["route"] != "unmatched" || records[0]["status"] != float64(404) {
		t.Fatalf("unexpected logs: %+v", records)
	}
	if strings.Contains(output.String(), "private-unmatched-path") {
		t.Error("unknown request path logged instead of a stable route label")
	}
}

func TestRecoverLogsPanic(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	router := chi.NewRouter()
	router.Use(middleware.Recover(logger), middleware.RequestLogging(logger))
	router.Get("/panic", func(http.ResponseWriter, *http.Request) { panic("private panic reason") })

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/panic", nil))

	if rec.Code != http.StatusInternalServerError || rec.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatalf("status=%d headers=%v body=%s", rec.Code, rec.Header(), rec.Body.String())
	}
	var problem struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if problem.Code != "internal_error" || strings.Contains(rec.Body.String(), "private panic reason") {
		t.Errorf("unexpected problem: %s", rec.Body.String())
	}

	records := decodeLogs(t, &output)
	requestLogs, panicLogs := 0, 0
	for _, record := range records {
		if record["level"] != "ERROR" {
			t.Errorf("panic record level=%v", record["level"])
		}
		switch record["msg"] {
		case "request handled":
			requestLogs++
			if record["status"] != float64(500) || record["route"] != "/panic" {
				t.Errorf("request record=%+v", record)
			}
		case "request panicked":
			panicLogs++
			if stack, ok := record["stack"].(string); !ok || !strings.Contains(stack, "TestRecoverLogsPanic") {
				t.Errorf("missing stack: %+v", record)
			}
		}
	}
	if requestLogs != 1 || panicLogs != 1 {
		t.Errorf("request logs=%d panic logs=%d, want 1 each", requestLogs, panicLogs)
	}
}

func decodeLogs(t *testing.T, output *bytes.Buffer) []map[string]any {
	t.Helper()

	var result []map[string]any
	for _, line := range bytes.Split(bytes.TrimSpace(output.Bytes()), []byte("\n")) {
		var record map[string]any
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatalf("invalid JSON log: %v; output=%s", err, output.String())
		}
		result = append(result, record)
	}

	return result
}
