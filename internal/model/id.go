package model

import (
	"github.com/google/uuid"

	"github.com/infrashift/chit/internal/assert"
)

// NewID generates a new UUIDv7 string.
func NewID() string {
	id := uuid.Must(uuid.NewV7()).String()
	assert.That(len(id) == 36, "NewID: id is not 36 characters")
	assert.That(id[14] == '7', "NewID: id is not a version 7 UUID")
	return id
}

// IsValidID checks whether s is a valid UUID string.
func IsValidID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}
