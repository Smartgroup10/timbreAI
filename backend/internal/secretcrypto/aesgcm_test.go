package secretcrypto

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func TestRoundtrip(t *testing.T) {
	key := make([]byte, KeyLength)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	for _, plain := range [][]byte{
		[]byte(""),
		[]byte("sk-test-1234567890abcdef"),
		[]byte("contains\nnewlines\tandñon-ascii"),
	} {
		ct, err := Encrypt(key, plain)
		if err != nil {
			t.Fatalf("encrypt %q: %v", plain, err)
		}
		if len(plain) > 0 && bytes.Contains(ct, plain) {
			t.Fatalf("ciphertext leaks plaintext: %x contains %q", ct, plain)
		}
		pt, err := Decrypt(key, ct)
		if err != nil {
			t.Fatalf("decrypt: %v", err)
		}
		if !bytes.Equal(pt, plain) {
			t.Fatalf("got %q want %q", pt, plain)
		}
	}
}

func TestWrongKeyFails(t *testing.T) {
	k1 := make([]byte, KeyLength)
	k2 := make([]byte, KeyLength)
	rand.Read(k1)
	rand.Read(k2)
	ct, err := Encrypt(k1, []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decrypt(k2, ct); err == nil {
		t.Fatal("expected decrypt with wrong key to fail")
	}
}

func TestDecodeKeyAcceptsStdAndRaw(t *testing.T) {
	raw := make([]byte, KeyLength)
	rand.Read(raw)
	cases := []string{
		base64.StdEncoding.EncodeToString(raw),
		base64.RawStdEncoding.EncodeToString(raw),
	}
	for _, c := range cases {
		got, err := DecodeKey(c)
		if err != nil {
			t.Fatalf("DecodeKey(%q): %v", c, err)
		}
		if !bytes.Equal(got, raw) {
			t.Fatalf("mismatch for %q", c)
		}
	}
}

func TestDecodeKeyRejectsBadLength(t *testing.T) {
	// 16 bytes — too short for AES-256.
	short := base64.StdEncoding.EncodeToString(make([]byte, 16))
	if _, err := DecodeKey(short); err == nil {
		t.Fatal("expected error for 16-byte key")
	}
	if _, err := DecodeKey(""); err == nil {
		t.Fatal("expected error for empty key")
	}
}
