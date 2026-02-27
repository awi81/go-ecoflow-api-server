package handlers

import (
	"net/http"
	"regexp"
	"unicode/utf8"
)

// SerialNumberValidator validates serial number path parameters
type SerialNumberValidator struct {
	// MinLength is the minimum allowed length for serial numbers
	MinLength int
	// MaxLength is the maximum allowed length for serial numbers
	MaxLength int
	// Pattern is an optional regex pattern for additional validation
	Pattern string
}

// DefaultSerialNumberValidator returns a validator with reasonable defaults
func DefaultSerialNumberValidator() SerialNumberValidator {
	return SerialNumberValidator{
		MinLength: 8,
		MaxLength: 32,
		Pattern:   "^[A-Za-z0-9_-]+$",
	}
}

// ValidateSerialNumber checks if a serial number is valid
// Returns true if valid, false otherwise
func (v SerialNumberValidator) ValidateSerialNumber(sn string) bool {
	// Check length
	length := utf8.RuneCountInString(sn)
	if length < v.MinLength || length > v.MaxLength {
		return false
	}

	// Check pattern if provided
	if v.Pattern != "" {
		matched, err := regexp.MatchString(v.Pattern, sn)
		if err != nil || !matched {
			return false
		}
	}

	return true
}

// SerialNumberValidationMiddleware returns a middleware that validates serial number path parameters
func SerialNumberValidationMiddleware(validator SerialNumberValidator, paramName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sn := r.PathValue(paramName)
			if sn != "" && !validator.ValidateSerialNumber(sn) {
				http.Error(w, "Invalid serial number format", http.StatusBadRequest)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
