package uploads

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// newUUID returns a random (version 4) UUID in the canonical hyphenated form.
// Twelve lines here save a dependency for the one thing we need it for.
func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate uuid: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32], nil
}

// StorageName builds the on-disk name for an upload: a generated UUID plus an
// extension derived from the resolved content type. Nothing the client sent
// contributes to it, so it can never contain a separator or a traversal.
func StorageName(contentType string) (string, error) {
	id, err := newUUID()
	if err != nil {
		return "", err
	}
	return id + "." + ExtFor(contentType), nil
}
