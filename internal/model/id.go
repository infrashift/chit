package model

import "github.com/google/uuid"

// NewID generates a new UUIDv7 string.
func NewID() string {
	return uuid.Must(uuid.NewV7()).String()
}

// IsValidID checks whether s is a valid UUID string.
func IsValidID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}
