package randstr

import (
	"crypto/rand"
	"encoding/hex"
)

func New(byteSize int) string {
	b := make([]byte, byteSize)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
