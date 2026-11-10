package trip_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/maksimkasimovhse/TripGo/internal/generated"
	"github.com/maksimkasimovhse/TripGo/internal/http/trip"
	"go.uber.org/mock/gomock"
)

func TestHealth(t *testing.T) {
	t.Parallel()

	readiness := NewMockReadinessChecker(gomock.NewController(t))

	h := trip.New(slog.New(slog.DiscardHandler), nil, nil, nil, readiness, time.Second)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	h.Health(rec, req)

	checkHealthResponse(t, rec, http.StatusOK, "ok")
}

func TestReady(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		pingErr    error
		status     int
		bodyStatus string
	}{
		{
			name:       "available",
			pingErr:    nil,
			status:     http.StatusOK,
			bodyStatus: "ok",
		},
		{
			name:       "unavailable",
			pingErr:    errors.New("password=private"),
			status:     http.StatusServiceUnavailable,
			bodyStatus: "unavailable",
		},
		{
			name:       "timeout",
			pingErr:    context.DeadlineExceeded,
			status:     http.StatusServiceUnavailable,
			bodyStatus: "unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			readiness := NewMockReadinessChecker(gomock.NewController(t))

			h := trip.New(slog.New(slog.DiscardHandler), nil, nil, nil, readiness, time.Second)

			req := httptest.NewRequest(http.MethodGet, "/ready", nil)
			rec := httptest.NewRecorder()

			var pingCtx context.Context

			readiness.EXPECT().
				Ping(gomock.Any()).
				DoAndReturn(func(ctx context.Context) error {
					pingCtx = ctx
					deadline, ok := ctx.Deadline()

					if !ok {
						t.Fatal("readiness context has no deadline")
					}
					if deadline.After(time.Now().Add(time.Second)) {
						t.Error("readiness deadline exceeds configured timeout")
					}
					return tt.pingErr
				})

			h.Ready(rec, req)

			checkHealthResponse(t, rec, tt.status, tt.bodyStatus)

			if !errors.Is(pingCtx.Err(), context.Canceled) {
				t.Errorf("Ping context not canceled after handler: %v", pingCtx.Err())
			}
		})
	}
}

func checkHealthResponse(t *testing.T, rec *httptest.ResponseRecorder, status int, bodyStatus string) {
	t.Helper()

	if rec.Code != status {
		t.Fatalf("status=%d want=%d body=%s", rec.Code, status, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type=%q", got)
	}

	var body api.HealthResponse

	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if string(body.Status) != bodyStatus {
		t.Errorf("Status=%q want=%q", body.Status, bodyStatus)
	}
	if strings.Contains(rec.Body.String(), "password=private") {
		t.Error("internal error leaked")
	}
}
