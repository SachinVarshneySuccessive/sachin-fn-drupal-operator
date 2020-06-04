package testhelpers

import (
	"github.com/google/uuid"
)

// IsValidUUID returns true if the provided string is valid uuid
func IsValidUUID(u string) bool {
	_, err := uuid.Parse(u)
	return err == nil
}
