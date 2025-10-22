package cryptocore

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha512"
	"errors"
	"io"
	"sync"

	"golang.org/x/crypto/curve25519"
)

var (
	randMu        sync.RWMutex
	randomnessSrc io.Reader = randReader{}
)

// randReader wraps crypto/rand.Reader but keeps the type unexported so tests can
// substitute deterministic sources.
type randReader struct{}

func (randReader) Read(p []byte) (int, error) {
	return rand.Read(p)
}

// UseDeterministicRandom swaps the randomness source for deterministic testing
// and returns a restore function that must be called when the test completes.
func UseDeterministicRandom(r io.Reader) func() {
	randMu.Lock()
	prev := randomnessSrc
	randomnessSrc = r
	randMu.Unlock()
	return func() {
		randMu.Lock()
		randomnessSrc = prev
		randMu.Unlock()
	}
}

func readRandom(b []byte) error {
	randMu.RLock()
	src := randomnessSrc
	randMu.RUnlock()
	_, err := io.ReadFull(src, b)
	return err
}

// GenerateIdentityKeypair creates a new device identity consisting of an
// Ed25519 signing key pair and the corresponding X25519 key material used for
// Diffie-Hellman operations.
func GenerateIdentityKeypair() (*Device, error) {
	seed := make([]byte, ed25519.SeedSize)
	if err := readRandom(seed); err != nil {
		return nil, err
	}
	priv := ed25519.NewKeyFromSeed(seed)
	pub := priv.Public().(ed25519.PublicKey)

	var dhPriv [32]byte
	dhPriv = ed25519PrivToCurve25519(priv)
	dhPubSlice, err := curve25519.X25519(dhPriv[:], curve25519.Basepoint)
	if err != nil {
		return nil, err
	}
	var dhPub [32]byte
	copy(dhPub[:], dhPubSlice)

	dev := &Device{
		identity: identityKeyPair{
			signingPublic:  append(ed25519.PublicKey(nil), pub...),
			signingPrivate: append(ed25519.PrivateKey(nil), priv...),
			dhPrivate:      dhPriv,
			dhPublic:       dhPub,
		},
		oneTime:   make(map[uint32]oneTimeEntry),
		nextOTKID: 1,
	}
	if err := dev.rotateSignedPrekey(); err != nil {
		return nil, err
	}
	return dev, nil
}

func (d *Device) rotateSignedPrekey() error {
	kp, err := generateX25519KeyPair()
	if err != nil {
		return err
	}
	sig := ed25519.Sign(d.identity.signingPrivate, kp.Public[:])
	d.signedPrekey = kp
	d.signedSig = append([]byte(nil), sig...)
	return nil
}

// PublishPrekeyBundle generates a signed prekey bundle with the requested number
// of fresh one-time prekeys. The bundle contains only public material and can be
// shared with other devices.
func (d *Device) PublishPrekeyBundle(oneTimeCount int) (*PrekeyBundle, error) {
	if d == nil {
		return nil, errors.New("cryptocore: nil device")
	}
	if len(d.signedPrekey.Public) == 0 {
		if err := d.rotateSignedPrekey(); err != nil {
			return nil, err
		}
	}
	bundle := &PrekeyBundle{
		IdentityKey:          d.identity.dhPublic,
		IdentitySignatureKey: append([]byte(nil), d.identity.signingPublic...),
		SignedPrekey:         d.signedPrekey.Public,
		SignedPrekeySig:      append([]byte(nil), d.signedSig...),
	}
	if oneTimeCount < 0 {
		oneTimeCount = 0
	}
	if oneTimeCount > 0 {
		bundle.OneTimePrekeys = make([]OneTimePrekey, 0, oneTimeCount)
	}
	for i := 0; i < oneTimeCount; i++ {
		kp, err := generateX25519KeyPair()
		if err != nil {
			return nil, err
		}
		id := d.nextOTKID
		d.nextOTKID++
		d.oneTime[id] = oneTimeEntry{key: kp}
		bundle.OneTimePrekeys = append(bundle.OneTimePrekeys, OneTimePrekey{ID: id, Public: kp.Public})
	}
	return bundle, nil
}

// IdentityPublic returns the static public keys for the device.
func (d *Device) IdentityPublic() (dh [32]byte, signing ed25519.PublicKey) {
	if d == nil {
		return [32]byte{}, nil
	}
	return d.identity.dhPublic, append(ed25519.PublicKey(nil), d.identity.signingPublic...)
}

func ed25519PrivToCurve25519(priv ed25519.PrivateKey) [32]byte {
	h := sha512.Sum512(priv.Seed())
	h[0] &= 248
	h[31] &= 127
	h[31] |= 64
	var out [32]byte
	copy(out[:], h[:32])
	return out
}

func generateX25519KeyPair() (keyPair, error) {
	var priv [32]byte
	if err := readRandom(priv[:]); err != nil {
		return keyPair{}, err
	}
	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64
	pub, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		return keyPair{}, err
	}
	var kp keyPair
	kp.Private = priv
	copy(kp.Public[:], pub)
	return kp, nil
}

var _ io.Reader = randReader{}
