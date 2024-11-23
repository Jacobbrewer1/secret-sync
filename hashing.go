package main

import (
	"crypto/sha1"
	"encoding/base64"
	"fmt"
)

func shaHash(data []byte) string {
	hasher := sha1.New()
	hasher.Write(data)
	return fmt.Sprintf("%x", hasher.Sum(nil))
}
