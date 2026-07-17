package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestResolveGracefulShutdownTimeout(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		expected time.Duration
		valid    bool
	}{
		{name: "default", expected: defaultGracefulShutdownTimeout, valid: true},
		{name: "configured", raw: "45s", expected: 45 * time.Second, valid: true},
		{name: "invalid", raw: "later", expected: defaultGracefulShutdownTimeout, valid: false},
		{name: "non-positive", raw: "0s", expected: defaultGracefulShutdownTimeout, valid: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, valid := resolveGracefulShutdownTimeout(test.raw)
			if actual != test.expected || valid != test.valid {
				t.Fatalf("resolveGracefulShutdownTimeout(%q) = (%s, %t), want (%s, %t)",
					test.raw, actual, valid, test.expected, test.valid)
			}
		})
	}
}

func reserveLocalAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve local address: %v", err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("release local address: %v", err)
	}
	return address
}

func startBlockingRequest(
	t *testing.T,
	address string,
	handlerStarted <-chan struct{},
) <-chan error {
	t.Helper()
	requestDone := make(chan error, 1)
	go func() {
		client := &http.Client{Timeout: 5 * time.Second}
		for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
			response, err := client.Get("http://" + address)
			if err != nil {
				select {
				case <-handlerStarted:
					requestDone <- err
					return
				default:
				}
				time.Sleep(5 * time.Millisecond)
				continue
			}
			requestDone <- response.Body.Close()
			return
		}
		requestDone <- errors.New("HTTP server did not accept the test request")
	}()

	select {
	case <-handlerStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("HTTP handler did not start")
	}
	return requestDone
}

func TestServeHTTPUntilShutdownWaitsForActiveRequest(t *testing.T) {
	address := reserveLocalAddress(t)
	handlerStarted := make(chan struct{})
	releaseHandler := make(chan struct{})
	applicationServer := &http.Server{
		Addr: address,
		Handler: http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
			close(handlerStarted)
			<-releaseHandler
			responseWriter.WriteHeader(http.StatusNoContent)
		}),
		ReadHeaderTimeout: time.Second,
	}
	shutdownSignal := make(chan struct{})
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- serveHTTPUntilShutdown(applicationServer, shutdownSignal, time.Second)
	}()
	requestDone := startBlockingRequest(t, address, handlerStarted)

	close(shutdownSignal)
	select {
	case err := <-serveDone:
		t.Fatalf("server returned before the active request drained: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	close(releaseHandler)

	if err := <-serveDone; err != nil {
		t.Fatalf("serveHTTPUntilShutdown() error = %v", err)
	}
	if err := <-requestDone; err != nil {
		t.Fatalf("active request failed: %v", err)
	}
}

func TestServeHTTPUntilShutdownForcesCloseAfterTimeout(t *testing.T) {
	address := reserveLocalAddress(t)
	handlerStarted := make(chan struct{})
	releaseHandler := make(chan struct{})
	applicationServer := &http.Server{
		Addr: address,
		Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			close(handlerStarted)
			<-releaseHandler
		}),
		ReadHeaderTimeout: time.Second,
	}
	shutdownSignal := make(chan struct{})
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- serveHTTPUntilShutdown(applicationServer, shutdownSignal, 50*time.Millisecond)
	}()
	requestDone := startBlockingRequest(t, address, handlerStarted)
	close(shutdownSignal)

	select {
	case err := <-serveDone:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("serveHTTPUntilShutdown() error = %v, want context deadline exceeded", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not force-close after the shutdown timeout")
	}
	close(releaseHandler)
	<-requestDone
}
