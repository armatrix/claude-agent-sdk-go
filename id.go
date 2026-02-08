package agent

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// ID prefix constants for different entity types.
const (
	PrefixSession = "sess"
	PrefixAgent   = "agt"
	PrefixRun     = "run"
	PrefixTeam    = "team"
)

// generateID produces a unique identifier with the given prefix and embedded timestamp.
// Format: {prefix}_{YYYYMMDDTHHmmss}_{16 hex chars}  e.g. "sess_20260208T150405_a1b2c3d4e5f6a7b8"
func generateID(prefix string) string {
	ts := time.Now().UTC().Format("20060102T150405")
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return prefix + "_" + ts + "_" + hex.EncodeToString(b)
}
