package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestValidateFields(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		message string
		ok      bool
	}{
		{name: "Ada", email: "ada@example.com", message: "Hello", ok: true},
		{name: "  Ada  ", email: " ada@example.com ", message: " Hello ", ok: true},
		{name: "", email: "ada@example.com", message: "Hello", ok: false},
		{name: "Ada", email: "not-an-email", message: "Hello", ok: false},
		{name: "Ada", email: "ada@example", message: "Hello", ok: false},
		{name: "Ada", email: "ada@example.com", message: "", ok: false},
		{name: "Ada\nLovelace", email: "ada@example.com", message: "Hello", ok: false},
		{name: strings.Repeat("a", 101), email: "ada@example.com", message: "Hello", ok: false},
		{name: "Ada", email: "ada@example.com", message: strings.Repeat("m", 2001), ok: false},
	}

	for _, tc := range tests {
		_, _, _, err := validateFields(tc.name, tc.email, tc.message)
		if tc.ok && err != nil {
			t.Errorf("validate(%q, %q, %q) unexpected error: %v", tc.name, tc.email, tc.message, err)
		}
		if !tc.ok && err == nil {
			t.Errorf("validate(%q, %q, %q) expected an error", tc.name, tc.email, tc.message)
		}
	}
}

func TestValidateFieldsTrims(t *testing.T) {
	name, email, message, err := validateFields("  Ada  ", " ada@example.com ", "  Hello  ")
	if err != nil {
		t.Fatal(err)
	}
	if name != "Ada" || email != "ada@example.com" || message != "Hello" {
		t.Fatalf("trimmed values = %q %q %q", name, email, message)
	}
}

func TestObjectKey(t *testing.T) {
	now := time.Date(2026, 9, 29, 10, 45, 12, 0, time.UTC)
	got := objectKey(now, "a83f21")
	want := "submissions/2026-09-29T10-45-12Z-a83f21.json"
	if got != want {
		t.Fatalf("objectKey() = %q, want %q", got, want)
	}
}

func TestObjectKeyUsesUTC(t *testing.T) {
	ist := time.FixedZone("IST", 5*3600+30*60)
	now := time.Date(2026, 9, 29, 16, 15, 12, 0, ist)
	got := objectKey(now, "a83f21")
	want := "submissions/2026-09-29T10-45-12Z-a83f21.json"
	if got != want {
		t.Fatalf("objectKey() = %q, want %q", got, want)
	}
}

func TestNewIDUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		id, err := newID()
		if err != nil {
			t.Fatal(err)
		}
		if len(id) != 6 {
			t.Fatalf("id length = %d, want 6 (%q)", len(id), id)
		}
		if seen[id] {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = true
	}
}

func TestSubmissionJSON(t *testing.T) {
	now := time.Date(2026, 9, 29, 10, 45, 12, 0, time.UTC)
	sub := buildSubmission("Ada", "ada@example.com", "Hello seminar", "a83f21", now)
	body, err := marshalSubmission(sub)
	if err != nil {
		t.Fatal(err)
	}

	var decoded Submission
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ID != "a83f21" || decoded.Name != "Ada" || decoded.Email != "ada@example.com" || decoded.Message != "Hello seminar" {
		t.Fatalf("decoded submission = %+v", decoded)
	}
	if !decoded.Timestamp.Equal(now) {
		t.Fatalf("timestamp = %s, want %s", decoded.Timestamp, now)
	}
	if !strings.Contains(string(body), "\n  ") {
		t.Fatalf("expected indented JSON, got %s", body)
	}
}
