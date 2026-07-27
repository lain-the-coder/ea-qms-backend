package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"unicode"
)

const (
	passwordMinLength  = 8
	passwordMinLower   = 1
	passwordMinUpper   = 1
	passwordMinDigit   = 1
	passwordMinSpecial = 1
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
