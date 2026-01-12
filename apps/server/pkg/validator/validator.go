package validator

import (
	"errors"
	"regexp"
	"strings"
	"unicode"
)

var (
	emailRegex          = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	ErrInvalidEmail     = errors.New("invalid email format")
	ErrWeakPassword     = errors.New("password must be at least 8 characters with uppercase, lowercase, digit, and special character")
	ErrEmptyName        = errors.New("name cannot be empty")
	ErrNameTooLong      = errors.New("name exceeds maximum length of 100 characters")
	ErrPasswordTooShort = errors.New("password must be at least 8 characters")
	ErrPasswordTooLong  = errors.New("password exceeds maximum length of 128 characters")
)

const (
	MinPasswordLength = 8
	MaxPasswordLength = 128
	MaxNameLength     = 100
)

func ValidateEmail(email string) (string, error) {
	email = strings.TrimSpace(email)
	email = strings.ToLower(email)

	if !emailRegex.MatchString(email) {
		return "", ErrInvalidEmail
	}

	return email, nil
}

func ValidatePassword(password string) error {
	if len(password) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	if len(password) > MaxPasswordLength {
		return ErrPasswordTooLong
	}

	var (
		hasUpper   bool
		hasLower   bool
		hasDigit   bool
		hasSpecial bool
	)

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsDigit(char):
			hasDigit = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	if !hasUpper || !hasLower || !hasDigit || !hasSpecial {
		return ErrWeakPassword
	}

	return nil
}

func ValidateName(name string) error {
	name = strings.TrimSpace(name)

	if name == "" {
		return ErrEmptyName
	}

	if len(name) > MaxNameLength {
		return ErrNameTooLong
	}

	return nil
}

func SanitizeName(name string) string {
	name = strings.TrimSpace(name)

	name = regexp.MustCompile(`\s+`).ReplaceAllString(name, " ")

	return name
}
