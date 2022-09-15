//go:build integration
// +build integration

package integration

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/bitrise-io/go-utils/v2/log"
)

var logger = log.NewLogger()

func checksumOf(bytes []byte) string {
	hash := sha256.New()
	hash.Write(bytes)
	return hex.EncodeToString(hash.Sum(nil))
}
