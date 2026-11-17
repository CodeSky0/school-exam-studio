package ai

import (
	"net/http"
	"testing"
)

func TestClassifyStatus(t *testing.T) {
	cases := []struct {
		status int
		want   ErrorKind
	}{
		{http.StatusUnauthorized, ErrorKindConfig},
		{http.StatusForbidden, ErrorKindConfig},
		{http.StatusNotFound, ErrorKindConfig},
		{http.StatusTooManyRequests, ErrorKindRetryable},
		{http.StatusInternalServerError, ErrorKindRetryable},
		{http.StatusBadGateway, ErrorKindRetryable},
		{http.StatusGatewayTimeout, ErrorKindRetryable},
		{http.StatusBadRequest, ErrorKindFatal},
		{http.StatusUnprocessableEntity, ErrorKindFatal},
	}
	for _, tc := range cases {
		if got := classifyStatus(tc.status); got != tc.want {
			t.Fatalf("status %d: got %s want %s", tc.status, got, tc.want)
		}
	}
}

func TestRetryAfterFromHeader(t *testing.T) {
	header := http.Header{}
	header.Set("Retry-After", "3")
	if got := retryAfterFromHeader(header); got.Seconds() != 3 {
		t.Fatalf("got %v want 3s", got)
	}
	header.Set("Retry-After", "not-a-number")
	if got := retryAfterFromHeader(header); got != 0 {
		t.Fatalf("got %v want 0", got)
	}
}
