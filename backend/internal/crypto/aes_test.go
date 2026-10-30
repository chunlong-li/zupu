package crypto

import (
	"testing"
)

func TestEncryptDecryptRoundtrip(t *testing.T) {
	key := GenerateKey()
	plain := "13812345678"

	enc, err := Encrypt(plain, key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if len(enc) < 12 {
		t.Fatalf("ciphertext too short: %d bytes", len(enc))
	}

	dec, err := Decrypt(enc, key)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if dec != plain {
		t.Fatalf("roundtrip mismatch: got %q, want %q", dec, plain)
	}
}

func TestEncryptEmpty(t *testing.T) {
	key := GenerateKey()
	enc, err := Encrypt("", key)
	if err != nil {
		t.Fatal(err)
	}
	if enc != "" {
		t.Fatalf("empty input should produce empty output, got %q", enc)
	}
}

func TestDecryptEmpty(t *testing.T) {
	key := GenerateKey()
	dec, err := Decrypt("", key)
	if err != nil {
		t.Fatal(err)
	}
	if dec != "" {
		t.Fatalf("empty input should produce empty output, got %q", dec)
	}
}

func TestDecryptWrongKey(t *testing.T) {
	key1 := GenerateKey()
	key2 := GenerateKey()
	enc, _ := Encrypt("hello", key1)
	_, err := Decrypt(enc, key2)
	if err == nil {
		t.Fatal("decrypt with wrong key should fail")
	}
}

func TestEncryptSamePlaintextDifferentCipher(t *testing.T) {
	key := GenerateKey()
	c1, _ := Encrypt("test12345678", key)
	c2, _ := Encrypt("test12345678", key)
	// AES-GCM with random nonce: same plaintext → different ciphertext
	if c1 == c2 {
		t.Fatal("two encryptions should produce different ciphertexts (random nonce)")
	}
}

func TestGenerateKeyLength(t *testing.T) {
	key := GenerateKey()
	b, _ := DecodeKey(key)
	if len(b) != 32 {
		t.Fatalf("key should be 32 bytes, got %d", len(b))
	}
}