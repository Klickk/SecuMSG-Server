package jwtsigner

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Signer holds an Ed25519 keypair for issuing JWTs.
type Signer struct {
	private ed25519.PrivateKey
	public  ed25519.PublicKey
	KeyID   string
	Issuer  string
}

// NewFromBase64 creates a signer from base64-encoded ed25519 private key bytes.
// If privB64 is empty, it generates an ephemeral key (good for local dev).
func NewFromBase64(privB64, kid, iss string) (*Signer, error) {
	var priv ed25519.PrivateKey
	if privB64 == "" {
		_, priv, _ = ed25519.GenerateKey(rand.Reader)
	} else {
		raw, err := base64.StdEncoding.DecodeString(privB64)
		if err != nil {
			return nil, err
		}
		if len(raw) != ed25519.PrivateKeySize {
			return nil, errors.New("invalid ed25519 private key size")
		}
		priv = ed25519.PrivateKey(raw)
	}
	pub := priv.Public().(ed25519.PublicKey)
	return &Signer{private: priv, public: pub, KeyID: kid, Issuer: iss}, nil
}

// Sign issues a JWT for subject `sub` with TTL and extra claims.
func (s *Signer) Sign(sub string, ttl time.Duration, claims map[string]any) (string, error) {
	now := time.Now()
	std := jwt.RegisteredClaims{
		Issuer:    s.Issuer,
		Subject:   sub,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
	}
	m := jwt.MapClaims{}
	for k, v := range claims {
		m[k] = v
	}
	m["iss"] = std.Issuer
	m["sub"] = std.Subject
	m["iat"] = std.IssuedAt.Unix()
	m["exp"] = std.ExpiresAt.Unix()

	t := jwt.NewWithClaims(jwt.SigningMethodEdDSA, m)
	t.Header["kid"] = s.KeyID
	return t.SignedString(s.private)
}

// PublicJWK renders the public part as JWK for JWKS endpoint.
func (s *Signer) PublicJWK() map[string]any {
	return map[string]any{
		"kty": "OKP",
		"crv": "Ed25519",
		"alg": "EdDSA",
		"use": "sig",
		"kid": s.KeyID,
		"x":   base64.RawURLEncoding.EncodeToString(s.public),
	}
}
