package typed

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAuthKeyPairRoundTripAndSignLogin(t *testing.T) {
	pair, err := GenerateAuthKeyPair()
	if err != nil {
		t.Fatalf("GenerateAuthKeyPair: %v", err)
	}
	if pair.PrivateKey == nil || pair.PrivateKey.Curve != elliptic.P256() {
		t.Fatalf("expected P256 private key")
	}
	if pair.PrivateKeyDERBase64 == "" || pair.PublicKeyDERBase64 == "" {
		t.Fatalf("expected base64 DER encodings")
	}

	priv, err := ParseAuthPrivateKeyBase64(pair.PrivateKeyDERBase64)
	if err != nil {
		t.Fatalf("ParseAuthPrivateKeyBase64: %v", err)
	}
	pub, err := ParseAuthPublicKeyBase64(pair.PublicKeyDERBase64)
	if err != nil {
		t.Fatalf("ParseAuthPublicKeyBase64: %v", err)
	}
	if !publicKeysEqual(pub, &priv.PublicKey) {
		t.Fatalf("parsed public key does not match private key")
	}

	got := string(LoginSignBytes(" dev-1 ", 5, 1700000000, " n1 "))
	want := "login\ndev-1\n5\n1700000000\nn1"
	if got != want {
		t.Fatalf("LoginSignBytes() = %q, want %q", got, want)
	}

	sig, err := SignLogin(priv, " dev-1 ", 5, 1700000000, " n1 ")
	if err != nil {
		t.Fatalf("SignLogin: %v", err)
	}
	rawSig, err := base64.StdEncoding.DecodeString(sig)
	if err != nil {
		t.Fatalf("signature is not base64: %v", err)
	}
	digest := sha256.Sum256(LoginSignBytes("dev-1", 5, 1700000000, "n1"))
	if !ecdsa.VerifyASN1(pub, digest[:], rawSig) {
		t.Fatalf("signature did not verify")
	}
}

func TestAuthKeyFileLoadOrCreate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "node_keys.json")
	pair, err := LoadOrCreateAuthKeyPair(path)
	if err != nil {
		t.Fatalf("LoadOrCreateAuthKeyPair create: %v", err)
	}
	if pair.PrivateKey == nil || pair.PublicKeyDERBase64 == "" {
		t.Fatalf("expected generated key pair")
	}

	readBack, err := ReadAuthKeyPair(path)
	if err != nil {
		t.Fatalf("ReadAuthKeyPair: %v", err)
	}
	if readBack.PublicKeyDERBase64 != pair.PublicKeyDERBase64 {
		t.Fatalf("public key changed after read")
	}

	loaded, err := LoadOrCreateAuthKeyPair(path)
	if err != nil {
		t.Fatalf("LoadOrCreateAuthKeyPair read: %v", err)
	}
	if loaded.PublicKeyDERBase64 != pair.PublicKeyDERBase64 {
		t.Fatalf("existing key was not reused")
	}
}

func TestAuthKeyFileInvalidDoesNotRegenerate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "node_keys.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"privkey":"bad","pubkey":"bad"}`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := LoadOrCreateAuthKeyPair(path); err == nil {
		t.Fatalf("expected invalid existing key file to fail")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(raw) != `{"privkey":"bad","pubkey":"bad"}` {
		t.Fatalf("invalid key file was overwritten: %s", string(raw))
	}
}

func TestAuthKeyValidationErrors(t *testing.T) {
	if _, err := GenerateAuthNonce(-1); !errors.Is(err, ErrAuthNonceSizeInvalid) {
		t.Fatalf("expected nonce size error, got %v", err)
	}
	nonce, err := GenerateAuthNonce(4)
	if err != nil {
		t.Fatalf("GenerateAuthNonce: %v", err)
	}
	if len(nonce) != 8 {
		t.Fatalf("expected 8 hex chars, got %q", nonce)
	}
	if _, err := hex.DecodeString(nonce); err != nil {
		t.Fatalf("nonce is not hex: %v", err)
	}

	p384, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatalf("generate P384: %v", err)
	}
	der, err := x509.MarshalECPrivateKey(p384)
	if err != nil {
		t.Fatalf("marshal P384: %v", err)
	}
	if _, err := ParseAuthPrivateKeyBase64(base64.StdEncoding.EncodeToString(der)); err == nil {
		t.Fatalf("expected non-P256 private key to fail")
	}

	if _, err := SignLogin(nil, "dev", 1, 1, "n"); !errors.Is(err, ErrPrivateKeyRequired) {
		t.Fatalf("expected private key error, got %v", err)
	}
}
