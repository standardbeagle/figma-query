package figma

import (
	"testing"
)

func TestNewClient(t *testing.T) {
	client := NewClient("test-token")
	if client == nil {
		t.Fatal("expected client to be created")
	}
	if client.accessToken != "test-token" {
		t.Errorf("expected token 'test-token', got '%s'", client.accessToken)
	}
	if client.baseURL != BaseURL {
		t.Errorf("expected baseURL '%s', got '%s'", BaseURL, client.baseURL)
	}
}

func TestAPIError(t *testing.T) {
	err := &APIError{Status: 404, Err: "Not Found"}
	expected := "figma API error (status 404): Not Found"
	if err.Error() != expected {
		t.Errorf("expected '%s', got '%s'", expected, err.Error())
	}
}

func TestRateLimitError(t *testing.T) {
	err := &RateLimitError{RetryAfter: "60"}
	if err.Error() != "rate limit exceeded, retry after: 60" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}
