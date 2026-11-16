package app

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestShutdownClosesResourcesBeforeStoppedLog(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	closed := false
	err := shutdown(ctx, logger, &http.Server{ReadHeaderTimeout: time.Second}, func(got context.Context) error {
		if got != ctx {
			t.Error("resource cleanup received a different shutdown context")
		}
		if strings.Contains(output.String(), "service stopped") {
			t.Error("service stopped was logged before resources closed")
		}
		closed = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !closed || !strings.Contains(output.String(), "service stopped") {
		t.Errorf("shutdown did not finish resource cleanup: %s", output.String())
	}
}

func TestShutdownReturnsWhenCleanupIgnoresDeadline(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	done := make(chan error, 1)
	logger := slog.New(slog.DiscardHandler)
	go func() {
		done <- shutdown(ctx, logger, &http.Server{ReadHeaderTimeout: time.Second}, func(context.Context) error {
			close(started)
			<-release
			close(finished)
			return nil
		})
	}()
	defer func() {
		close(release)
		<-finished
	}()

	<-started
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("shutdown error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown kept waiting for blocked cleanup after cancellation")
	}
}

func TestShutdownReturnsCleanupError(t *testing.T) {
	t.Parallel()

	want := errors.New("close resources failed")
	err := shutdown(context.Background(), slog.New(slog.DiscardHandler), &http.Server{ReadHeaderTimeout: time.Second}, func(context.Context) error {
		return want
	})
	if !errors.Is(err, want) {
		t.Errorf("shutdown error = %v, want %v", err, want)
	}
}
