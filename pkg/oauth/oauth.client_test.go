package oauth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGetUserInfoPropagatesContextAndPKCE(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	deadline, _ := ctx.Deadline()
	calls := 0
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		got, ok := r.Context().Deadline()
		if !ok || !got.Equal(deadline) {
			t.Fatalf("request deadline = %v, want %v", got, deadline)
		}
		body := `{"id":"subject","email":"student@example.test","verified_email":true}`
		if r.URL.Path == "/token" {
			data, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), "code_verifier=verifier") {
				t.Fatalf("PKCE absent: %s", data)
			}
			body = `{"access_token":"access","token_type":"Bearer"}`
		} else if r.Header.Get("Authorization") != "Bearer access" {
			t.Fatal("userinfo access token absent")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	httpClient := &http.Client{Transport: transport}
	previous := http.DefaultClient
	http.DefaultClient = httpClient
	t.Cleanup(func() { http.DefaultClient = previous })
	ctx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)
	cfg := &oauth2.Config{ClientID: "client", Endpoint: oauth2.Endpoint{TokenURL: "https://provider.example.test/token", AuthStyle: oauth2.AuthStyleInParams}}
	result, err := NewGoogleOAuthClient(cfg, zap.NewNop()).GetUserInfo(ctx, "code", "verifier")
	if err != nil || result.ID != "subject" || !result.VerifiedEmail || calls != 2 {
		t.Fatalf("result=%+v, calls=%d, error=%v", result, calls, err)
	}
}

func TestGetUserInfoRetainsCancellationCause(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) { return nil, r.Context().Err() })}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, client)
	cfg := &oauth2.Config{Endpoint: oauth2.Endpoint{TokenURL: "https://provider.example.test/token", AuthStyle: oauth2.AuthStyleInParams}}
	_, err := NewGoogleOAuthClient(cfg, zap.NewNop()).GetUserInfo(ctx, "code", "verifier")
	if !errors.Is(err, ErrInvalidCode) || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation cause lost: %v", err)
	}
}
