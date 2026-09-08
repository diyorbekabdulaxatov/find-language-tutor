package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// argon2id parameters. Starting point from the OWASP Password Storage Cheat
// Sheet ("m=64MiB, t=1, p=4"): tune upward as hardware allows. The values are
// encoded into every hash (PHC string), so raising them here does not break
// verification of existing hashes — only new hashes use the new cost.
const (
	argonMemoryKiB   = 64 * 1024 // 64 MiB
	argonTime        = 1         // iterations
	argonParallelism = 4         // lanes
	argonSaltLen     = 16        // bytes
	argonKeyLen      = 32        // bytes (256-bit derived key)
)

// errBadHash is returned by VerifyPassword when the stored string is not a hash
// this code produced. It is not exported: callers only care true/false + err.
var errBadHash = errors.New("auth: malformed password hash")

// HashPassword returns a PHC-format argon2id hash string:
//
//	$argon2id$v=19$m=65536,t=1,p=4$<b64 salt>$<b64 key>
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("auth: read salt: %w", err)
	}

	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemoryKiB, argonParallelism, argonKeyLen)

	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemoryKiB, argonTime, argonParallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyPassword reports whether password matches the given PHC hash string,
// using the cost parameters embedded in that string and a constant-time
// comparison. A non-nil error means the stored hash was unreadable (treat as a
// non-match and log).
func VerifyPassword(password, encoded string) (bool, error) {
	p, salt, want, err := decodeHash(encoded)
	if err != nil {
		return false, err
	}

	got := argon2.IDKey([]byte(password), salt, p.time, p.memory, p.parallelism, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

type argonParams struct {
	memory      uint32
	time        uint32
	parallelism uint8
}

func decodeHash(encoded string) (argonParams, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	// ["", "argon2id", "v=19", "m=..,t=..,p=..", "<salt>", "<key>"]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return argonParams{}, nil, nil, errBadHash
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return argonParams{}, nil, nil, errBadHash
	}

	var p argonParams
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.time, &p.parallelism); err != nil {
		return argonParams{}, nil, nil, errBadHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return argonParams{}, nil, nil, errBadHash
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return argonParams{}, nil, nil, errBadHash
	}

	return p, salt, key, nil
}
