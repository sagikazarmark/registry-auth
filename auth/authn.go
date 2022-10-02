package auth

import (
	"context"
	"errors"
)

// ErrAuthenticationFailed is returned when authentication fails.
//
// This error should only be returned if credential verification fails.
// Any other error (eg. connection problems) should be returned directly.
var ErrAuthenticationFailed = errors.New("authentication failed")

// PasswordAuthenticator authenticates a subject using the "password" grant or basic auth.
//
// It returns an ErrAuthenticationFailed error in case credentials are invalid.
type PasswordAuthenticator interface {
	Authenticate(ctx context.Context, username string, password string) (Subject, error)
}
