package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// newID returns a sortable, collision-resistant id with the given prefix:
// `<prefix>_<unixnano>_<hex>` — readable in logs, unique across creates within the
// same nanosecond thanks to the 4-byte random suffix.
func newID(prefix string) string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%s_%d_%s", prefix, time.Now().UTC().UnixNano(), hex.EncodeToString(b[:]))
}
