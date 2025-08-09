package utils

import (
	"crypto/sha256"
	"encoding/hex"
)

func hashTGID(tgid string) string {
	hash := sha256.Sum256([]byte(tgid))
	return hex.EncodeToString(hash[:])
}
