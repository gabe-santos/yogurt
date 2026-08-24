// Package auth owns the reader's credentials: the configured password, browser
// sessions, and the rate limit that protects login.
package auth

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// Password is the configured password, hashed at startup and never held in
// plaintext beyond that call.
type Password struct {
	hash []byte
}

// NewPassword hashes the configured password.
func NewPassword(plain string) (*Password, error) {
	if plain == "" {
		return nil, fmt.Errorf("password is empty")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	return &Password{hash: hash}, nil
}

// Matches reports whether a candidate is the configured password.
func (p *Password) Matches(candidate string) bool {
	if candidate == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword(p.hash, []byte(candidate)) == nil
}
