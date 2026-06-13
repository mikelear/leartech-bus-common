// Package random provides utilities for generating cryptographically secure random values.
//
// The package offers a RandomnessGenerator interface and a default Generator implementation
// for creating random UUIDs and alphanumeric strings using crypto/rand for secure randomness.
package random

import (
	"crypto/rand"
	"github.com/google/uuid"
)

// RandomnessGenerator defines an interface for generating random values.
// Implementations provide methods for creating UUIDs and random strings.
type RandomnessGenerator interface {
	// NewUUID generates a new random UUID v4.
	NewUUID() uuid.UUID
	// NewString generates a cryptographically secure random alphanumeric string
	// of the specified length. Returns an error if the underlying random source fails.
	NewString(length int) (string, error)
}

// Generator implements RandomnessGenerator using crypto/rand for secure random generation.
// It generates alphanumeric strings from a character set of [a-zA-Z0-9].
type Generator struct {
	letters []byte
}

// NewGenerator creates a new Generator instance configured with the default
// alphanumeric character set (a-z, A-Z, 0-9).
func NewGenerator() *Generator {
	return &Generator{
		letters: []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"),
	}
}

// NewUUID generates a new random UUID version 4 using the google/uuid package.
func (g *Generator) NewUUID() uuid.UUID {
	return uuid.New()
}

// NewString generates a cryptographically secure random alphanumeric string of the
// specified length. The string contains only characters from the set [a-zA-Z0-9].
//
// Returns an error if crypto/rand fails to generate random bytes.
func (g *Generator) NewString(length int) (string, error) {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	for i := range b {
		b[i] = g.letters[int(b[i])%len(g.letters)]
	}
	return string(b), nil
}
