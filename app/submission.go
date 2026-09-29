package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// Submission is the JSON document stored as one S3 object.
type Submission struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Message   string    `json:"message"`
}

func validateFields(name, email, message string) (string, string, string, error) {
	name = strings.TrimSpace(name)
	email = strings.TrimSpace(email)
	message = strings.TrimSpace(message)

	switch {
	case name == "":
		return "", "", "", errors.New("Name is required.")
	case len([]rune(name)) > 100:
		return "", "", "", errors.New("Name must be 100 characters or fewer.")
	case !singleLine(name):
		return "", "", "", errors.New("Name must be a single line.")
	case !validEmail(email):
		return "", "", "", errors.New("Enter a valid email address.")
	case message == "":
		return "", "", "", errors.New("Message is required.")
	case len([]rune(message)) > 2000:
		return "", "", "", errors.New("Message must be 2000 characters or fewer.")
	case strings.ContainsRune(message, '\x00'):
		return "", "", "", errors.New("Message contains an invalid character.")
	}
	return name, email, message, nil
}

func singleLine(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func validEmail(email string) bool {
	if len(email) < 3 || len(email) > 254 || strings.ContainsAny(email, " \t\r\n") {
		return false
	}
	at := strings.LastIndex(email, "@")
	if at <= 0 || at >= len(email)-1 {
		return false
	}
	domain := email[at+1:]
	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") || !strings.Contains(domain, ".") {
		return false
	}
	return true
}

func newID() (string, error) {
	buf := make([]byte, 3)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func buildSubmission(name, email, message, id string, now time.Time) Submission {
	return Submission{
		ID:        id,
		Timestamp: now.UTC(),
		Name:      name,
		Email:     email,
		Message:   message,
	}
}

// objectKey returns submissions/<utc-timestamp>-<id>.json.
// Colons in the timestamp are replaced so the key is easy to copy.
func objectKey(now time.Time, id string) string {
	stamp := now.UTC().Format("2006-01-02T15-04-05Z")
	return fmt.Sprintf("submissions/%s-%s.json", stamp, id)
}

func marshalSubmission(sub Submission) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(sub); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
