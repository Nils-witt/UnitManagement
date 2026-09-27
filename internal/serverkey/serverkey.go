// Package serverkey gives this instance a persistent RSA key pair of its own,
// stored in the keys directory. The private key signs this server's sync
// requests to other instances (see internal/sync); administrators register
// the public key there as an API key.
package serverkey

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// keyBits is the RSA modulus size of a generated key pair.
const keyBits = 4096

// PrivateKeyFileName and PublicKeyFileName are the files EnsureKeyPair reads
// and writes in the keys directory.
const (
	PrivateKeyFileName = "server.key"
	PublicKeyFileName  = "server.pub"
)

// EnsureLoadPrivateKey ensures dir holds a key pair (see EnsureKeyPair) and
// returns its private key.
func EnsureLoadPrivateKey(dir string) (*rsa.PrivateKey, error) {
	if err := EnsureKeyPair(dir); err != nil {
		return nil, fmt.Errorf("ensure server key pair: %w", err)
	}
	key, err := LoadPrivateKey(dir)
	if err != nil {
		return nil, fmt.Errorf("load server key pair: %w", err)
	}
	return key, nil
}

// EnsureKeyPair makes sure dir contains server.key and server.pub, creating
// dir and generating a 4096-bit pair (PEM-encoded PKCS #8 private key and
// PKIX public key) if server.key doesn't exist yet. An existing server.key is
// never changed, so this is safe on every startup.
func EnsureKeyPair(dir string) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create keys dir: %w", err)
	}

	privatePath := filepath.Join(dir, PrivateKeyFileName)
	if _, err := os.Stat(privatePath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", privatePath, err)
	}

	key, err := rsa.GenerateKey(rand.Reader, keyBits)
	if err != nil {
		return fmt.Errorf("generate server key pair: %w", err)
	}
	privDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return fmt.Errorf("encode server private key: %w", err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return fmt.Errorf("encode server public key: %w", err)
	}

	// The public key is written first: a crash in between then leaves no
	// server.key, so the next startup generates a fresh, matching pair.
	publicPath := filepath.Join(dir, PublicKeyFileName)
	publicPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
	if err := os.WriteFile(publicPath, publicPEM, 0o644); err != nil { //nolint:gosec // a public key is not a secret
		return fmt.Errorf("write %s: %w", publicPath, err)
	}
	privatePEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER})
	if err := os.WriteFile(privatePath, privatePEM, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", privatePath, err)
	}
	return nil
}

// LoadPrivateKey reads the private key EnsureKeyPair wrote to dir.
func LoadPrivateKey(dir string) (*rsa.PrivateKey, error) {
	privatePath := filepath.Join(dir, PrivateKeyFileName)
	pemBytes, err := os.ReadFile(privatePath) //nolint:gosec // dir is operator configuration, not request input
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", privatePath, err)
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("%s: not a valid PEM file", privatePath)
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", privatePath, err)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New(privatePath + ": not an RSA private key")
	}
	return rsaKey, nil
}

// PublicKeyPEM returns the PEM-encoded public key of key, as registered on
// other instances.
func PublicKeyPEM(key *rsa.PrivateKey) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return "", fmt.Errorf("encode server public key: %w", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})), nil
}
