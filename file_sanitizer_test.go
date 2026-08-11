package main

import (
	"strings"
	"testing"
)

func TestSanitizeFilename(t *testing.T) {
	longFilename := strings.Repeat("a", 300) + ".pdf"
	expectedLongFilename := strings.Repeat("a", 251) + ".pdf" // 251 'a's + 4 '.pdf' = 255 runes

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Drop quotes and semicolons",
			input:    `report"; drop.pdf`,
			expected: "report drop.pdf",
		},
		{
			name:     "Strip path traversal",
			input:    `..\..\..\etc\evidence.pdf`,
			expected: "evidence.pdf",
		},
		{
			name:     "Cap long filename to 255 characters",
			input:    longFilename,
			expected: expectedLongFilename,
		},
		{
			name:     "Fallback on empty string",
			input:    "",
			expected: "evidence.pdf",
		},
		{
			name:     "Fallback on dot pdf only",
			input:    ".pdf",
			expected: "evidence.pdf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeFilename(tt.input)
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}
