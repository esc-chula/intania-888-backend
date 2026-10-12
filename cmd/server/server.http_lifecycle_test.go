package server

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

type httpRequestResult struct {
	status int
	body   string
	err    error
}

func newShutdownTestServer(t *testing.T, handler fiber.Handler) (*FiberHTTPServer, string, <-chan error) {
	t.Helper()

	app := fiber.New(fiber.Config{
		ReadTimeout: time.Second,
		IdleTimeout: time.Second,
	})
	app.Get("/active", handler)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	t.Cleanup(func() {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Errorf("listener.Close() error = %v", err)
		}
	})

	listenDone := make(chan error, 1)
	go func() {
		listenDone <- app.Listener(listener)
	}()

	return &FiberHTTPServer{app: app}, "http://" + listener.Addr().String() + "/active", listenDone
}

func requestHTTP(client *http.Client, url string) <-chan httpRequestResult {
	result := make(chan httpRequestResult, 1)
	go func() {
		response, err := client.Get(url)
		if err != nil {
			result <- httpRequestResult{err: err}

			return
		}
		body, err := io.ReadAll(response.Body)
		closeErr := response.Body.Close()
		if err == nil {
			err = closeErr
		}
		result <- httpRequestResult{
			status: response.StatusCode,
			body:   string(body),
			err:    err,
		}
	}()

	return result
}

func awaitRequest(t *testing.T, result <-chan httpRequestResult) httpRequestResult {
	t.Helper()

	select {
	case got := <-result:
		return got
	case <-time.After(2 * time.Second):
		t.Fatal("HTTP request did not finish")

		return httpRequestResult{}
	}
}

func awaitListener(t *testing.T, listenDone <-chan error) {
	t.Helper()

	select {
	case err := <-listenDone:
		if err != nil {
			t.Errorf("app.Listener() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Fiber listener did not stop")
	}
}

func TestFiberHTTPServerShutdownWaitsForActiveRequest(t *testing.T) {
	requestStarted := make(chan struct{})
	releaseRequest := make(chan struct{})
	var releaseOnce sync.Once
	finishRequest := func() {
		releaseOnce.Do(func() {
			close(releaseRequest)
		})
	}

	defer finishRequest()

	httpServer, url, listenDone := newShutdownTestServer(t, func(c *fiber.Ctx) error {
		close(requestStarted)
		<-releaseRequest

		return c.SendString("completed")
	})
	client := &http.Client{Timeout: 2 * time.Second}
	defer client.CloseIdleConnections()
	requestDone := requestHTTP(client, url)

	select {
	case <-requestStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("request handler did not start")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	shutdownDone := make(chan error, 1)
	go func() {
		shutdownDone <- httpServer.shutdown(ctx)
	}()

	select {
	case err := <-shutdownDone:
		t.Fatalf("shutdown returned before the active request finished: %v", err)
	case <-time.After(25 * time.Millisecond):
	}

	finishRequest()
	request := awaitRequest(t, requestDone)
	if request.err != nil {
		t.Fatalf("HTTP request error = %v", request.err)
	}
	if request.status != http.StatusOK || request.body != "completed" {
		t.Fatalf("HTTP response = (%d, %q), want (%d, %q)", request.status, request.body, http.StatusOK, "completed")
	}

	select {
	case err := <-shutdownDone:
		if err != nil {
			t.Fatalf("shutdown error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown did not finish after the active request completed")
	}
	awaitListener(t, listenDone)
}

func TestFiberHTTPServerShutdownHonorsDeadline(t *testing.T) {
	requestStarted := make(chan struct{})
	releaseRequest := make(chan struct{})
	var releaseOnce sync.Once
	finishRequest := func() {
		releaseOnce.Do(func() {
			close(releaseRequest)
		})
	}

	defer finishRequest()

	httpServer, url, listenDone := newShutdownTestServer(t, func(c *fiber.Ctx) error {
		close(requestStarted)
		<-releaseRequest

		return c.SendString("completed")
	})
	client := &http.Client{Timeout: 2 * time.Second}
	defer client.CloseIdleConnections()
	requestDone := requestHTTP(client, url)

	select {
	case <-requestStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("request handler did not start")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	shutdownDone := make(chan error, 1)
	go func() {
		shutdownDone <- httpServer.shutdown(ctx)
	}()

	var err error
	select {
	case err = <-shutdownDone:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not return after its deadline")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("shutdown error = %v, want context deadline exceeded", err)
	}

	finishRequest()
	request := awaitRequest(t, requestDone)
	if request.err != nil {
		t.Fatalf("HTTP request error = %v", request.err)
	}
	if request.status != http.StatusOK || !strings.Contains(request.body, "completed") {
		t.Fatalf("HTTP response = (%d, %q), want completed response", request.status, request.body)
	}
	awaitListener(t, listenDone)
}
