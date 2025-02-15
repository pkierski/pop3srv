package pop3srv

import (
	"crypto/md5"
	"encoding/hex"
)

// ApopVerify is a helper function implements APOP
// authentication. It can be used for implementing [Authorizer.Apop].
func ApopVerify(timestampBanner, digest, password string) bool {
	hash := md5.Sum([]byte(timestampBanner + password))
	return hex.EncodeToString(hash[:]) == digest
}
