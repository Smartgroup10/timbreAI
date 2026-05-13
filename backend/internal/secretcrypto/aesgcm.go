// Package secretcrypto cifra secretos guardados en BD (API keys de proveedores
// de voz, etc.) con AES-256-GCM. La master key viene de la env SECRETS_MASTER_KEY
// (32 bytes base64) — si rota, los blobs viejos dejan de ser legibles.
//
// Llamamos al package "secretcrypto" en vez de "crypto" para no chocar con el
// stdlib package y para que sea obvio en imports.
package secretcrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// KeyLength es el tamaño esperado de SECRETS_MASTER_KEY tras decodificar base64.
// 32 bytes = AES-256.
const KeyLength = 32

// DecodeKey acepta SECRETS_MASTER_KEY como base64 (raw o std) y devuelve los 32 bytes.
// Si la longitud es != 32, error. Pensado para llamarse una vez al arrancar.
func DecodeKey(s string) ([]byte, error) {
	if s == "" {
		return nil, errors.New("SECRETS_MASTER_KEY is required (base64 of 32 random bytes)")
	}
	// Probamos primero std, luego raw — aceptamos ambos para que `openssl rand
	// -base64 32` o `openssl rand -base64 32 | tr -d =` funcionen ambos.
	for _, dec := range []func(string) ([]byte, error){
		base64.StdEncoding.DecodeString,
		base64.RawStdEncoding.DecodeString,
		base64.URLEncoding.DecodeString,
		base64.RawURLEncoding.DecodeString,
	} {
		if b, err := dec(s); err == nil && len(b) == KeyLength {
			return b, nil
		}
	}
	return nil, fmt.Errorf("SECRETS_MASTER_KEY must decode to %d bytes (got something else)", KeyLength)
}

// Encrypt cifra plaintext con AES-256-GCM. Devuelve nonce || ciphertext || tag
// como []byte (la API estándar de GCM ya lo empaqueta así con Seal).
func Encrypt(key, plaintext []byte) ([]byte, error) {
	if len(key) != KeyLength {
		return nil, fmt.Errorf("key must be %d bytes", KeyLength)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	// El formato resultante (nonce || sealed) se guarda tal cual en BYTEA.
	out := gcm.Seal(nonce, nonce, plaintext, nil)
	return out, nil
}

// Decrypt revierte Encrypt. Si la key no coincide o el blob está corrupto, error.
func Decrypt(key, blob []byte) ([]byte, error) {
	if len(key) != KeyLength {
		return nil, fmt.Errorf("key must be %d bytes", KeyLength)
	}
	if len(blob) == 0 {
		return nil, nil
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	if len(blob) < ns {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ct := blob[:ns], blob[ns:]
	return gcm.Open(nil, nonce, ct, nil)
}
