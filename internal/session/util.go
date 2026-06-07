package session

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// randomString generates a random hex string of the specified length.
func randomString(length int) string {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if random fails
		return hex.EncodeToString([]byte(time.Now().Format("20060102150405.000000")))[:length]
	}
	return hex.EncodeToString(bytes)[:length]
}

// now returns the current time in UTC.
func now() time.Time {
	return time.Now().UTC()
}
