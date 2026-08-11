package main

import (
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// sanitizeFilename cleans an untrusted filename for headers and database storage.
func sanitizeFilename(filename string) string {
	// normalize Windows backslashes to forward slashes so filepath.Base works on Linux/WSL
	normalized := strings.ReplaceAll(filename, "\\", "/")

	// strip directory path
	safe := filepath.Base(normalized)

	// strip control characters, quotes, and header-breaking delimiters
	safe = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 || r == '"' || r == '\'' || r == '`' || r == ';' {
			return -1 // Drop character
		}
		return r
	}, safe)

	safe = strings.TrimSpace(safe)

	// include "." in fallback check (filepath.Base("") returns ".")
	if safe == "" || safe == "." || safe == ".pdf" {
		return "evidence.pdf"
	}

	// cap length to 255 runes while preserving the extension
	if utf8.RuneCountInString(safe) > 255 {
		ext := filepath.Ext(safe)
		stemStr := safe[:len(safe)-len(ext)]
		stemRunes := []rune(stemStr)

		maxStemLen := 255 - utf8.RuneCountInString(ext)
		if len(stemRunes) > maxStemLen {
			stemRunes = stemRunes[:maxStemLen]
		}
		safe = string(stemRunes) + ext
	}
	return safe
}
