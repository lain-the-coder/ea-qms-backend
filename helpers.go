package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"unicode"
)

const (
	passwordMinLength  = 8
	passwordMinLower   = 1
	passwordMinUpper   = 1
	passwordMinDigit   = 1
	passwordMinSpecial = 1
	defaultPageLimit   = 50
	maxPageLimit       = 200
)

// declaring error response struct globally for free use
type errorResponse struct {
	Error string `json:"error"`
}

// generic helper function for error construction
func respondWithError(w http.ResponseWriter, msg string, statusCode int) {
	errorBody := errorResponse{}
	errorBody.Error = msg
	// delegating json construction to helper function
	respondWithJSON(w, statusCode, errorBody)
}

// generic helper function for json response construction
func respondWithJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}
	w.WriteHeader(statusCode)
	w.Write(data)
}

// validatePassword returns a list of unmet requirements — empty means the
// password is acceptable. All rules are collected rather than failing on the
// first, so the caller gets one complete message.
func validatePassword(password string) []string {
	var lower, upper, digit, special int
	for _, ch := range password {
		switch {
		case unicode.IsLower(ch):
			lower++
		case unicode.IsUpper(ch):
			upper++
		case unicode.IsDigit(ch):
			digit++
		case unicode.IsPunct(ch), unicode.IsSymbol(ch):
			special++
		}
	}

	problems := []string{}
	if len([]rune(password)) < passwordMinLength {
		problems = append(problems, fmt.Sprintf("at least %d characters", passwordMinLength))
	}
	if lower < passwordMinLower {
		problems = append(problems, fmt.Sprintf("at least %d lowercase letter", passwordMinLower))
	}
	if upper < passwordMinUpper {
		problems = append(problems, fmt.Sprintf("at least %d uppercase letter", passwordMinUpper))
	}
	if digit < passwordMinDigit {
		problems = append(problems, fmt.Sprintf("at least %d digit", passwordMinDigit))
	}
	if special < passwordMinSpecial {
		problems = append(problems, fmt.Sprintf("at least %d special character", passwordMinSpecial))
	}
	return problems
}

func parsePagination(q url.Values) (int32, int32, error) {
	// default limit is 50
	limit := int32(defaultPageLimit)
	if limitStr := q.Get("limit"); limitStr != "" {
		// Parse specifically as a 32-bit integer
		l, err := strconv.ParseInt(limitStr, 10, 32)
		if err != nil || l < 1 {
			return 0, 0, fmt.Errorf("invalid limit: must be a positive integer")
		}
		if l > maxPageLimit {
			l = maxPageLimit
		}
		limit = int32(l)
	}

	// default offset is 0
	offset := int32(0)
	if offsetStr := q.Get("offset"); offsetStr != "" {
		// Parse specifically as a 32-bit integer
		o, err := strconv.ParseInt(offsetStr, 10, 32)
		if err != nil || o < 0 {
			return 0, 0, fmt.Errorf("invalid offset: must be a non-negative integer")
		}
		offset = int32(o)
	}

	return limit, offset, nil
}

func boolValue(b *bool) bool {
	if b != nil {
		return *b
	}
	return false
}

// strPtr returns a pointer to s. Needed because Go does not allow taking the
// address of a literal, and the nullable audit columns are *string.
func strPtr(s string) *string {
	return &s
}
