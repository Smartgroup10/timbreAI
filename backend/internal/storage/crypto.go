package storage

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// derive implements the AWS Signature V4 key derivation chain.
func derive(secretKey, dateStamp, region, service string) []byte {
	k1 := hmacSHA256([]byte("AWS4"+secretKey), []byte(dateStamp))
	k2 := hmacSHA256(k1, []byte(region))
	k3 := hmacSHA256(k2, []byte(service))
	return hmacSHA256(k3, []byte("aws4_request"))
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func sha256Sum(b []byte) []byte {
	s := sha256.Sum256(b)
	return s[:]
}

// hex is a small alias so storage.go can read more naturally.
func hexStr(b []byte) string { return hex.EncodeToString(b) }
