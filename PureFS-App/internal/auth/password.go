package auth

import (
	"errors"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrPasswordTooShort        = errors.New("password must be at least 8 characters")
	ErrPasswordNoUpper         = errors.New("password must contain at least one uppercase letter")
	ErrPasswordNoLower         = errors.New("password must contain at least one lowercase letter")
	ErrPasswordNoDigit         = errors.New("password must contain at least one digit")
	ErrPasswordNoSpecial       = errors.New("password must contain at least one special character")
)

// ValidatePasswordStrength checks that the given password meets the policy requirements.
// The policy is read from the provided PasswordPolicyConfig.
func ValidatePasswordStrength(password string, minLength int, requireUpper, requireLower, requireDigit, requireSpecial bool) error {
	if len(password) < minLength {
		return ErrPasswordTooShort
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, c := range password {
		switch {
		case unicode.IsUpper(c):
			hasUpper = true
		case unicode.IsLower(c):
			hasLower = true
		case unicode.IsDigit(c):
			hasDigit = true
		case unicode.IsPunct(c) || unicode.IsSymbol(c):
			hasSpecial = true
		}
	}

	if requireUpper && !hasUpper {
		return ErrPasswordNoUpper
	}
	if requireLower && !hasLower {
		return ErrPasswordNoLower
	}
	if requireDigit && !hasDigit {
		return ErrPasswordNoDigit
	}
	if requireSpecial && !hasSpecial {
		return ErrPasswordNoSpecial
	}

	return nil
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
