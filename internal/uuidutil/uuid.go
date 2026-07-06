// Package uuidutil generates random UUIDs without a third-party dependency.
package uuidutil

import (
	"crypto/rand"
	"encoding/hex"
)

// New returns a random RFC-4122 v4 UUID string. Used to tag an ephemeral CLI
// session so successive interactive turns can share backend context.
func New() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10

	s := hex.EncodeToString(b[:])
	return s[0:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:32], nil
}
