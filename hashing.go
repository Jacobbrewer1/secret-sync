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

func base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func base64Decode(data string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(data)
}
