package cryptocore

import (
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"io"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

const hkdfInfoX3DH = "SecuMSG-X3DH"

// InitSession performs the X3DH handshake as the initiator using the remote
// prekey bundle and prepares the initial Double Ratchet state.
func (d *Device) InitSession(bundle *PrekeyBundle) (*SessionState, *HandshakeMessage, error) {
	if d == nil {
		return nil, nil, errors.New("cryptocore: nil device")
	}
	if bundle == nil {
		return nil, nil, errors.New("cryptocore: nil bundle")
	}
	if err := verifyPrekeyBundle(bundle); err != nil {
		return nil, nil, err
	}

	ephemeral, err := generateX25519KeyPair()
	if err != nil {
		return nil, nil, err
	}

	var otk *OneTimePrekey
	if len(bundle.OneTimePrekeys) > 0 {
		otk = &bundle.OneTimePrekeys[0]
	}

	secret, err := deriveSharedSecretInitiator(d, bundle, ephemeral, otk)
	if err != nil {
		return nil, nil, err
	}
	root, chain := deriveInitialKeys(secret)

	var pending *uint32
	if otk != nil {
		pending = new(uint32)
		*pending = otk.ID
	}

	sess := &SessionState{
		RootKey:         root,
		SendChain:       chainState{Key: chain},
		RecvChain:       chainState{},
		RatchetPrivate:  ephemeral.Private,
		RatchetPublic:   ephemeral.Public,
		RemoteRatchet:   bundle.SignedPrekey,
		RemoteIdentity:  bundle.IdentityKey,
		RemoteSignature: append([]byte(nil), bundle.IdentitySignatureKey...),
		Role:            RoleInitiator,
		PendingPrekey:   pending,
		skipped:         make(map[string][32]byte),
	}

	msg := &HandshakeMessage{
		IdentityKey:          d.identity.dhPublic,
		IdentitySignatureKey: append([]byte(nil), d.identity.signingPublic...),
		EphemeralKey:         ephemeral.Public,
		OneTimePrekeyID:      pending,
	}
	return sess, msg, nil
}

// AcceptSession finalizes the X3DH handshake on the responder side using the
// initiator's handshake message and prepares the receiving Double Ratchet state.
func (d *Device) AcceptSession(msg *HandshakeMessage) (*SessionState, error) {
	if d == nil {
		return nil, errors.New("cryptocore: nil device")
	}
	if msg == nil {
		return nil, errors.New("cryptocore: nil handshake message")
	}
	var otk *keyPair
	if msg.OneTimePrekeyID != nil {
		entry, ok := d.oneTime[*msg.OneTimePrekeyID]
		if !ok {
			return nil, ErrMissingOneTimeKey
		}
		k := entry.key
		otk = &k
		delete(d.oneTime, *msg.OneTimePrekeyID)
	}
	secret, err := deriveSharedSecretResponder(d, msg, otk)
	if err != nil {
		return nil, err
	}
	root, chain := deriveInitialKeys(secret)

	sess := &SessionState{
		RootKey:         root,
		SendChain:       chainState{},
		RecvChain:       chainState{Key: chain},
		RatchetPrivate:  d.signedPrekey.Private,
		RatchetPublic:   d.signedPrekey.Public,
		RemoteRatchet:   msg.EphemeralKey,
		RemoteIdentity:  msg.IdentityKey,
		RemoteSignature: append([]byte(nil), msg.IdentitySignatureKey...),
		Role:            RoleResponder,
		PendingPrekey:   msg.OneTimePrekeyID,
		skipped:         make(map[string][32]byte),
	}
	return sess, nil
}

func verifyPrekeyBundle(bundle *PrekeyBundle) error {
	if len(bundle.IdentitySignatureKey) != ed25519.PublicKeySize {
		return ErrInvalidPrekeySignature
	}
	if !ed25519.Verify(ed25519.PublicKey(bundle.IdentitySignatureKey), bundle.SignedPrekey[:], bundle.SignedPrekeySig) {
		return ErrInvalidPrekeySignature
	}
	return nil
}

func deriveSharedSecretInitiator(d *Device, bundle *PrekeyBundle, eph keyPair, otk *OneTimePrekey) ([]byte, error) {
	dh1, err := curve25519.X25519(d.identity.dhPrivate[:], bundle.SignedPrekey[:])
	if err != nil {
		return nil, err
	}
	dh2, err := curve25519.X25519(eph.Private[:], bundle.IdentityKey[:])
	if err != nil {
		return nil, err
	}
	dh3, err := curve25519.X25519(eph.Private[:], bundle.SignedPrekey[:])
	if err != nil {
		return nil, err
	}
	secret := append(append(append([]byte{}, dh1...), dh2...), dh3...)
	if otk != nil {
		dh4, err := curve25519.X25519(eph.Private[:], otk.Public[:])
		if err != nil {
			return nil, err
		}
		secret = append(secret, dh4...)
	}
	return secret, nil
}

func deriveSharedSecretResponder(d *Device, msg *HandshakeMessage, otk *keyPair) ([]byte, error) {
	dh1, err := curve25519.X25519(d.signedPrekey.Private[:], msg.IdentityKey[:])
	if err != nil {
		return nil, err
	}
	dh2, err := curve25519.X25519(d.identity.dhPrivate[:], msg.EphemeralKey[:])
	if err != nil {
		return nil, err
	}
	dh3, err := curve25519.X25519(d.signedPrekey.Private[:], msg.EphemeralKey[:])
	if err != nil {
		return nil, err
	}
	secret := append(append(append([]byte{}, dh1...), dh2...), dh3...)
	if otk != nil {
		dh4, err := curve25519.X25519(otk.Private[:], msg.EphemeralKey[:])
		if err != nil {
			return nil, err
		}
		secret = append(secret, dh4...)
	}
	return secret, nil
}

func deriveInitialKeys(secret []byte) ([32]byte, [32]byte) {
	kdf := hkdf.New(sha256.New, secret, nil, []byte(hkdfInfoX3DH))
	var root, chain [32]byte
	if _, err := io.ReadFull(kdf, root[:]); err != nil {
		// return zero-value keys on failure
		return [32]byte{}, [32]byte{}
	}
	if _, err := io.ReadFull(kdf, chain[:]); err != nil {
		// return zero-value keys on failure
		return [32]byte{}, [32]byte{}
	}
	return root, chain
}
