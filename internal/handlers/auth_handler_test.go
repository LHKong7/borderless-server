package handlers

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"testing"
)

// buildPEMPrivateKey encodes an RSA private key to PKCS#1/PKCS#8 PEM.
func buildPEMPrivateKey(t *testing.T, key *rsa.PrivateKey) []byte {
	t.Helper()
	b := x509.MarshalPKCS1PrivateKey(key)
	blk := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: b}
	return pem.EncodeToMemory(blk)
}

// buildPayload returns 16 random bytes concatenated with SHA-256(password).
func buildPayload(t *testing.T, password string) []byte {
	t.Helper()
	rand16 := make([]byte, 16)
	if _, err := rand.Read(rand16); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	h := sha256.Sum256([]byte(password))
	return append(rand16, h[:]...)
}

func TestDecryptWithRSAPrivateKey_OAEP_SHA256_Success(t *testing.T) {
	// 1) Generate RSA keypair
	key, err := rsa.GenerateKey(rand.Reader, 1024)

	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	pemPriv := buildPEMPrivateKey(t, key)
	// fmt.Println("pemPriv is ...", pemPriv)

	// 2) Build payload (16 rand + sha256(password))
	payload := buildPayload(t, "admin12345")

	// 3) Encrypt with RSA-OAEP SHA-256 using public key
	cipherBytes, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, &key.PublicKey, payload, nil)
	if err != nil {
		t.Fatalf("EncryptOAEP: %v", err)
	}

	// 4) Decrypt via function under test
	plain, err := decryptWithRSAPrivateKey(pemPriv, cipherBytes)
	if err != nil {
		t.Fatalf("decryptWithRSAPrivateKey error: %v", err)
	}

	if !bytes.Equal(plain, payload) {
		t.Fatalf("decrypted payload mismatch: got %d bytes, want %d", len(plain), len(payload))
	}
}

func TestDecryptWithRSAPrivateKey_FailsWithWrongPadding(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	pemPriv := buildPEMPrivateKey(t, key)
	payload := buildPayload(t, "admin12345")

	// Encrypt with PKCS#1 v1.5 (wrong padding for our OAEP decrypt)
	cipherPKCS1, err := rsa.EncryptPKCS1v15(rand.Reader, &key.PublicKey, payload)
	if err != nil {
		t.Fatalf("EncryptPKCS1v15: %v", err)
	}

	if _, err := decryptWithRSAPrivateKey(pemPriv, cipherPKCS1); err == nil {
		t.Fatalf("expected error when decrypting PKCS#1 v1.5 with OAEP, got nil")
	}
}
