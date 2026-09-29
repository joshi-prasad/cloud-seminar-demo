package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type fakeStore struct {
	key  string
	body []byte
	err  error
}

func (f *fakeStore) PutJSON(ctx context.Context, key string, body []byte) error {
	if f.err != nil {
		return f.err
	}
	f.key = key
	f.body = bytes.Clone(body)
	return nil
}

func testApp(t *testing.T, store ObjectStore) *App {
	t.Helper()
	tmpl, err := loadTemplates()
	if err != nil {
		t.Fatal(err)
	}
	app := newApp(Config{
		AppName:    "cloud-seminar-demo",
		AppVersion: "1.0.0",
		Port:       "8080",
		AWSRegion:  "us-west-2",
		S3Bucket:   "prasad-cloud-demo",
	}, store, tmpl, slog.New(slog.NewTextHandler(io.Discard, nil)), "test-host")
	app.exit = func(int) {}
	app.now = func() time.Time {
		return time.Date(2026, 9, 29, 10, 45, 12, 0, time.UTC)
	}
	app.newID = func() (string, error) { return "a83f21", nil }
	return app
}

func TestHealth(t *testing.T) {
	app := testApp(t, &fakeStore{})
	rr := httptest.NewRecorder()
	app.routes().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rr.Code != http.StatusOK || rr.Body.String() != "ok\n" {
		t.Fatalf("health = %d %q", rr.Code, rr.Body.String())
	}
}

func TestReady(t *testing.T) {
	app := testApp(t, &fakeStore{})
	rr := httptest.NewRecorder()
	app.routes().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if rr.Code != http.StatusOK || rr.Body.String() != "ready\n" {
		t.Fatalf("ready = %d %q", rr.Code, rr.Body.String())
	}
}

func TestReadyWithoutStore(t *testing.T) {
	app := testApp(t, nil)
	rr := httptest.NewRecorder()
	app.routes().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready status = %d, want 503", rr.Code)
	}
}

func TestCrashDoesNotKillTestProcess(t *testing.T) {
	app := testApp(t, &fakeStore{})
	var code int
	called := false
	app.exit = func(c int) {
		called = true
		code = c
	}
	rr := httptest.NewRecorder()
	app.routes().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/crash", nil))
	if !called || code != 1 {
		t.Fatalf("exit called=%v code=%d", called, code)
	}
}

func TestInfo(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIA_SHOULD_NOT_APPEAR")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret-should-not-appear")

	app := testApp(t, &fakeStore{})
	req := httptest.NewRequest(http.MethodGet, "/info", nil)
	req.Header.Set("Authorization", "Bearer super-secret-token")
	rr := httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	body := rr.Body.String()
	for _, want := range []string{
		"name: cloud-seminar-demo\n",
		"version: 1.0.0\n",
		"hostname: test-host\n",
		"storage: s3\n",
		"region: us-west-2\n",
		"bucket: prasad-cloud-demo\n",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("info missing %q\nbody:\n%s", want, body)
		}
	}
	for _, secret := range []string{"AKIA_SHOULD_NOT_APPEAR", "secret-should-not-appear", "super-secret-token", "Authorization"} {
		if strings.Contains(body, secret) {
			t.Fatalf("info leaked %q\n%s", secret, body)
		}
	}
}

func TestIndexForm(t *testing.T) {
	app := testApp(t, &fakeStore{})
	rr := httptest.NewRecorder()
	app.routes().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	body := rr.Body.String()
	for _, part := range []string{`name="name"`, `name="email"`, `name="message"`, `action="/submit"`} {
		if !strings.Contains(body, part) {
			t.Fatalf("form missing %s", part)
		}
	}
}

func TestSubmitSuccess(t *testing.T) {
	store := &fakeStore{}
	app := testApp(t, store)
	rr := httptest.NewRecorder()
	app.routes().ServeHTTP(rr, formRequest("Ada Lovelace", "ada@example.com", "Hello from the seminar"))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	const key = "submissions/2026-09-29T10-45-12Z-a83f21.json"
	if store.key != key {
		t.Fatalf("stored key = %q", store.key)
	}
	if !strings.Contains(rr.Body.String(), key) {
		t.Fatalf("confirmation missing key: %s", rr.Body.String())
	}

	var sub Submission
	if err := json.Unmarshal(store.body, &sub); err != nil {
		t.Fatal(err)
	}
	if sub.ID != "a83f21" || sub.Name != "Ada Lovelace" || sub.Email != "ada@example.com" || sub.Message != "Hello from the seminar" {
		t.Fatalf("stored submission = %+v", sub)
	}
}

func TestSubmitValidation(t *testing.T) {
	store := &fakeStore{}
	app := testApp(t, store)
	rr := httptest.NewRecorder()
	app.routes().ServeHTTP(rr, formRequest("", "not-an-email", ""))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
	if store.key != "" {
		t.Fatal("invalid form was stored")
	}
	if !strings.Contains(rr.Body.String(), "Name is required.") {
		t.Fatalf("missing validation message: %s", rr.Body.String())
	}
}

func TestSubmitEscapesRedisplayedInput(t *testing.T) {
	app := testApp(t, &fakeStore{})
	rr := httptest.NewRecorder()
	app.routes().ServeHTTP(rr, formRequest(`<script>`, "not-an-email", "ok"))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
	body := rr.Body.String()
	if strings.Contains(body, "<script>") {
		t.Fatalf("raw script was redisplayed: %s", body)
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Fatalf("expected escaped name, got %s", body)
	}
}

func TestSubmitStoreFailureHidesInternals(t *testing.T) {
	secret := "AccessDenied AKIA_INTERNAL_KEY arn:aws:iam::123456789012:user/demo"
	app := testApp(t, &fakeStore{err: errors.New(secret)})
	rr := httptest.NewRecorder()
	app.routes().ServeHTTP(rr, formRequest("Ada", "ada@example.com", "Hello"))
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("status = %d", rr.Code)
	}
	body := rr.Body.String()
	if strings.Contains(body, secret) || strings.Contains(body, "AKIA") || strings.Contains(body, "AccessDenied") {
		t.Fatalf("error page leaked storage details: %s", body)
	}
	if !strings.Contains(body, "could not be saved") {
		t.Fatalf("missing friendly error: %s", body)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	app := testApp(t, &fakeStore{})
	rr := httptest.NewRecorder()
	app.routes().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("x")))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST / status = %d", rr.Code)
	}
}

func formRequest(name, email, message string) *http.Request {
	form := url.Values{}
	form.Set("name", name)
	form.Set("email", email)
	form.Set("message", message)
	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}
