package auth_test

import (
	"net/http"
	"testing"

	"github.com/lain-the-coder/ea-qms-backend/internal/auth"
)

func TestGetBearerToken(t *testing.T) {
	tests := []struct {
		name      string
		header    string // "" means the Authorization header is absent entirely
		setHeader bool
		want      string
		wantErr   bool
	}{
		{
			name:      "valid bearer token",
			header:    "Bearer abc123",
			setHeader: true,
			want:      "abc123",
			wantErr:   false,
		},
		{
			name:      "no authorization header at all",
			setHeader: false,
			wantErr:   true,
		},
		{
			name:      "empty authorization header",
			header:    "",
			setHeader: true,
			wantErr:   true,
		},
		{
			name:      "wrong scheme is rejected",
			header:    "Basic abc123",
			setHeader: true,
			wantErr:   true,
		},
		{
			name:      "bearer prefix with no token",
			header:    "Bearer ",
			setHeader: true,
			wantErr:   true,
		},
		{
			name:      "bearer prefix with only whitespace",
			header:    "Bearer    ",
			setHeader: true,
			wantErr:   true,
		},
		{
			name:      "surrounding whitespace is trimmed",
			header:    "Bearer   abc123   ",
			setHeader: true,
			want:      "abc123",
			wantErr:   false,
		},
		{
			// Deliberate: the scheme match is case-sensitive. RFC 7235 says the
			// scheme is case-insensitive, but every real client sends "Bearer",
			// and being strict is the safer default.
			name:      "lowercase bearer is rejected",
			header:    "bearer abc123",
			setHeader: true,
			wantErr:   true,
		},
		{
			name:      "token containing dots survives intact (JWTs have them)",
			header:    "Bearer eyJhbGciOi.eyJzdWIiOi.SflKxwRJSM",
			setHeader: true,
			want:      "eyJhbGciOi.eyJzdWIiOi.SflKxwRJSM",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := http.Header{}
			if tt.setHeader {
				headers.Set("Authorization", tt.header)
			}

			got, err := auth.GetBearerToken(headers)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected an error, got token %q", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got token %q, want %q", got, tt.want)
			}
		})
	}
}
