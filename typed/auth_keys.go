package typed

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	AuthAlgES256          = "ES256"
	DefaultAuthNonceBytes = 12
)

var (
	ErrDeviceIDRequired      = errors.New("device_id is required")
	ErrPrivateKeyRequired    = errors.New("private key is required")
	ErrPublicKeyRequired     = errors.New("pubkey is required")
	ErrTimestampRequired     = errors.New("ts is required")
	ErrNonceRequired         = errors.New("nonce is required")
	ErrUnsupportedAuthAlg    = errors.New("unsupported auth alg")
	ErrAuthKeyPathRequired   = errors.New("auth key path is required")
	ErrAuthNonceSizeInvalid  = errors.New("nonce byte length must be positive")
	ErrAuthKeyFileEmpty      = errors.New("auth key file is empty")
	ErrAuthKeyPairMismatched = errors.New("auth key pair public key does not match private key")
)

// AuthKeyPair matches the auth node key file format used by Server, Win, and Android.
type AuthKeyPair struct {
	PrivateKey          *ecdsa.PrivateKey
	PrivateKeyDERBase64 string
	PublicKeyDERBase64  string
}

type authKeyFile struct {
	PrivKey string `json:"privkey"`
	PubKey  string `json:"pubkey"`
}

// GenerateAuthKeyPair creates a P256 key pair and returns the base64 DER encodings.
func GenerateAuthKeyPair() (AuthKeyPair, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return AuthKeyPair{}, err
	}
	return AuthKeyPairFromPrivateKey(priv)
}

// AuthKeyPairFromPrivateKey derives the SDK auth key-pair representation from a P256 private key.
func AuthKeyPairFromPrivateKey(priv *ecdsa.PrivateKey) (AuthKeyPair, error) {
	if err := validateAuthPrivateKey(priv); err != nil {
		return AuthKeyPair{}, err
	}
	privB64, err := EncodeAuthPrivateKeyBase64(priv)
	if err != nil {
		return AuthKeyPair{}, err
	}
	pubB64, err := EncodeAuthPublicKeyBase64(&priv.PublicKey)
	if err != nil {
		return AuthKeyPair{}, err
	}
	return AuthKeyPair{
		PrivateKey:          priv,
		PrivateKeyDERBase64: privB64,
		PublicKeyDERBase64:  pubB64,
	}, nil
}

// ParseAuthPrivateKeyBase64 parses a base64 DER EC private key and requires P256.
func ParseAuthPrivateKeyBase64(b64 string) (*ecdsa.PrivateKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil {
		return nil, err
	}
	priv, err := x509.ParseECPrivateKey(raw)
	if err != nil {
		return nil, err
	}
	if err := validateAuthPrivateKey(priv); err != nil {
		return nil, err
	}
	return priv, nil
}

// ParseAuthPublicKeyBase64 parses a base64 DER PKIX public key and requires P256.
func ParseAuthPublicKeyBase64(b64 string) (*ecdsa.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil {
		return nil, err
	}
	pubAny, err := x509.ParsePKIXPublicKey(raw)
	if err != nil {
		return nil, err
	}
	pub, ok := pubAny.(*ecdsa.PublicKey)
	if !ok || pub == nil || pub.Curve != elliptic.P256() {
		return nil, errors.New("pubkey not p256")
	}
	return pub, nil
}

// EncodeAuthPrivateKeyBase64 encodes a P256 EC private key as base64 DER.
func EncodeAuthPrivateKeyBase64(priv *ecdsa.PrivateKey) (string, error) {
	if err := validateAuthPrivateKey(priv); err != nil {
		return "", err
	}
	der, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(der), nil
}

// EncodeAuthPublicKeyBase64 encodes a P256 ECDSA public key as base64 DER PKIX.
func EncodeAuthPublicKeyBase64(pub *ecdsa.PublicKey) (string, error) {
	if pub == nil || pub.Curve != elliptic.P256() {
		return "", errors.New("pubkey not p256")
	}
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(der), nil
}

// ReadAuthKeyPair reads a node key JSON file with fields privkey/pubkey.
func ReadAuthKeyPair(path string) (AuthKeyPair, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return AuthKeyPair{}, ErrAuthKeyPathRequired
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return AuthKeyPair{}, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return AuthKeyPair{}, ErrAuthKeyFileEmpty
	}
	var file authKeyFile
	if err := json.Unmarshal(data, &file); err != nil {
		return AuthKeyPair{}, err
	}
	return parseAuthKeyPair(file.PrivKey, file.PubKey)
}

// WriteAuthKeyPair writes a node key JSON file with restrictive permissions.
func WriteAuthKeyPair(path string, pair AuthKeyPair) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return ErrAuthKeyPathRequired
	}
	pair, err := normalizeAuthKeyPair(pair)
	if err != nil {
		return err
	}
	cleaned := filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(cleaned), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(authKeyFile{
		PrivKey: pair.PrivateKeyDERBase64,
		PubKey:  pair.PublicKeyDERBase64,
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cleaned, data, 0o600)
}

// LoadOrCreateAuthKeyPair reads an existing key file, or creates one when it is missing.
func LoadOrCreateAuthKeyPair(path string) (AuthKeyPair, error) {
	pair, err := ReadAuthKeyPair(path)
	if err == nil {
		return pair, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return AuthKeyPair{}, err
	}
	pair, err = GenerateAuthKeyPair()
	if err != nil {
		return AuthKeyPair{}, err
	}
	if err := WriteAuthKeyPair(path, pair); err != nil {
		return AuthKeyPair{}, err
	}
	return pair, nil
}

// GenerateAuthNonce returns a hex-encoded cryptographic nonce.
func GenerateAuthNonce(byteLen int) (string, error) {
	if byteLen == 0 {
		byteLen = DefaultAuthNonceBytes
	}
	if byteLen < 0 {
		return "", ErrAuthNonceSizeInvalid
	}
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// LoginSignBytes returns the exact byte string verified by the Auth SubProto.
func LoginSignBytes(deviceID string, nodeID uint32, ts int64, nonce string) []byte {
	sb := strings.Builder{}
	sb.WriteString("login\n")
	sb.WriteString(strings.TrimSpace(deviceID))
	sb.WriteString("\n")
	sb.WriteString(strconv.FormatUint(uint64(nodeID), 10))
	sb.WriteString("\n")
	sb.WriteString(strconv.FormatInt(ts, 10))
	sb.WriteString("\n")
	sb.WriteString(strings.TrimSpace(nonce))
	return []byte(sb.String())
}

// SignLogin signs an Auth login request using ES256 (P256 + SHA256 + ASN.1 ECDSA).
func SignLogin(priv *ecdsa.PrivateKey, deviceID string, nodeID uint32, ts int64, nonce string) (string, error) {
	if err := validateLoginSigningInputs(priv, deviceID, nodeID, ts, nonce); err != nil {
		return "", err
	}
	hashed := sha256.Sum256(LoginSignBytes(deviceID, nodeID, ts, nonce))
	sig, err := ecdsa.SignASN1(rand.Reader, priv, hashed[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

func parseAuthKeyPair(privB64, pubB64 string) (AuthKeyPair, error) {
	priv, err := ParseAuthPrivateKeyBase64(privB64)
	if err != nil {
		return AuthKeyPair{}, err
	}
	pub, err := ParseAuthPublicKeyBase64(pubB64)
	if err != nil {
		return AuthKeyPair{}, err
	}
	if !publicKeysEqual(pub, &priv.PublicKey) {
		return AuthKeyPair{}, ErrAuthKeyPairMismatched
	}
	return AuthKeyPair{
		PrivateKey:          priv,
		PrivateKeyDERBase64: strings.TrimSpace(privB64),
		PublicKeyDERBase64:  strings.TrimSpace(pubB64),
	}, nil
}

func normalizeAuthKeyPair(pair AuthKeyPair) (AuthKeyPair, error) {
	priv := pair.PrivateKey
	var err error
	if priv == nil && strings.TrimSpace(pair.PrivateKeyDERBase64) != "" {
		priv, err = ParseAuthPrivateKeyBase64(pair.PrivateKeyDERBase64)
		if err != nil {
			return AuthKeyPair{}, err
		}
	}
	if err := validateAuthPrivateKey(priv); err != nil {
		return AuthKeyPair{}, err
	}

	privB64 := strings.TrimSpace(pair.PrivateKeyDERBase64)
	if privB64 == "" {
		privB64, err = EncodeAuthPrivateKeyBase64(priv)
		if err != nil {
			return AuthKeyPair{}, err
		}
	}
	pubB64 := strings.TrimSpace(pair.PublicKeyDERBase64)
	if pubB64 == "" {
		pubB64, err = EncodeAuthPublicKeyBase64(&priv.PublicKey)
		if err != nil {
			return AuthKeyPair{}, err
		}
	} else {
		pub, err := ParseAuthPublicKeyBase64(pubB64)
		if err != nil {
			return AuthKeyPair{}, err
		}
		if !publicKeysEqual(pub, &priv.PublicKey) {
			return AuthKeyPair{}, ErrAuthKeyPairMismatched
		}
	}

	return AuthKeyPair{
		PrivateKey:          priv,
		PrivateKeyDERBase64: privB64,
		PublicKeyDERBase64:  pubB64,
	}, nil
}

func validateAuthPrivateKey(priv *ecdsa.PrivateKey) error {
	if priv == nil {
		return ErrPrivateKeyRequired
	}
	if priv.Curve != elliptic.P256() {
		return errors.New("private key not p256")
	}
	return nil
}

func validateLoginSigningInputs(priv *ecdsa.PrivateKey, deviceID string, nodeID uint32, ts int64, nonce string) error {
	if err := validateAuthPrivateKey(priv); err != nil {
		return err
	}
	if strings.TrimSpace(deviceID) == "" {
		return ErrDeviceIDRequired
	}
	if nodeID == 0 {
		return ErrNodeIDRequired
	}
	if ts == 0 {
		return ErrTimestampRequired
	}
	if strings.TrimSpace(nonce) == "" {
		return ErrNonceRequired
	}
	return nil
}

func publicKeysEqual(a, b *ecdsa.PublicKey) bool {
	if a == nil || b == nil {
		return false
	}
	return a.Curve == b.Curve && a.X.Cmp(b.X) == 0 && a.Y.Cmp(b.Y) == 0
}

func publicKeyFromPair(pair AuthKeyPair) (string, error) {
	if pub := strings.TrimSpace(pair.PublicKeyDERBase64); pub != "" {
		parsedPub, err := ParseAuthPublicKeyBase64(pub)
		if err != nil {
			return "", err
		}
		if pair.PrivateKey != nil && !publicKeysEqual(parsedPub, &pair.PrivateKey.PublicKey) {
			return "", ErrAuthKeyPairMismatched
		}
		if strings.TrimSpace(pair.PrivateKeyDERBase64) != "" {
			priv, err := ParseAuthPrivateKeyBase64(pair.PrivateKeyDERBase64)
			if err != nil {
				return "", err
			}
			if !publicKeysEqual(parsedPub, &priv.PublicKey) {
				return "", ErrAuthKeyPairMismatched
			}
		}
		return pub, nil
	}
	if pair.PrivateKey != nil {
		return EncodeAuthPublicKeyBase64(&pair.PrivateKey.PublicKey)
	}
	if strings.TrimSpace(pair.PrivateKeyDERBase64) != "" {
		priv, err := ParseAuthPrivateKeyBase64(pair.PrivateKeyDERBase64)
		if err != nil {
			return "", err
		}
		return EncodeAuthPublicKeyBase64(&priv.PublicKey)
	}
	return "", ErrPublicKeyRequired
}

func privateKeyFromPair(pair AuthKeyPair) (*ecdsa.PrivateKey, error) {
	if pair.PrivateKey != nil {
		return pair.PrivateKey, validateAuthPrivateKey(pair.PrivateKey)
	}
	if strings.TrimSpace(pair.PrivateKeyDERBase64) != "" {
		return ParseAuthPrivateKeyBase64(pair.PrivateKeyDERBase64)
	}
	return nil, ErrPrivateKeyRequired
}

func ensureES256(alg string) (string, error) {
	alg = strings.TrimSpace(alg)
	if alg == "" {
		return AuthAlgES256, nil
	}
	if !strings.EqualFold(alg, AuthAlgES256) {
		return "", fmt.Errorf("%w: %s", ErrUnsupportedAuthAlg, alg)
	}
	return AuthAlgES256, nil
}
