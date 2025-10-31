package cryptocore

import (
	"encoding/base64"
	"errors"
	"fmt"
)

type DeviceState struct {
	SigningPrivate  string                        `json:"signingPrivate"`
	SigningPublic   string                        `json:"signingPublic"`
	DHPrivate       string                        `json:"dhPrivate"`
	DHPublic        string                        `json:"dhPublic"`
	SignedPrekey    X25519KeyPairState            `json:"signedPrekey"`
	SignedPrekeySig string                        `json:"signedPrekeySig"`
	OneTime         map[uint32]X25519KeyPairState `json:"oneTime,omitempty"`
	NextOTKID       uint32                        `json:"nextOtkId"`
}

type X25519KeyPairState struct {
	Private string `json:"private"`
	Public  string `json:"public"`
}

type SessionStateSnapshot struct {
	RootKey         string             `json:"rootKey"`
	SendChain       ChainStateSnapshot `json:"sendChain"`
	RecvChain       ChainStateSnapshot `json:"recvChain"`
	RatchetPrivate  string             `json:"ratchetPrivate"`
	RatchetPublic   string             `json:"ratchetPublic"`
	RemoteRatchet   string             `json:"remoteRatchet"`
	RemoteIdentity  string             `json:"remoteIdentity"`
	RemoteSignature string             `json:"remoteSignature"`
	PN              uint32             `json:"pn"`
	Role            SessionRole        `json:"role"`
	PendingPrekey   *uint32            `json:"pendingPrekey,omitempty"`
	Skipped         map[string]string  `json:"skipped,omitempty"`
}

type ChainStateSnapshot struct {
	Key   string `json:"key"`
	Index uint32 `json:"index"`
}

func (d *Device) Export() (*DeviceState, error) {
	if d == nil {
		return nil, errors.New("cryptocore: nil device")
	}
	state := &DeviceState{
		SigningPrivate: base64.StdEncoding.EncodeToString(d.identity.signingPrivate),
		SigningPublic:  base64.StdEncoding.EncodeToString(d.identity.signingPublic),
		DHPrivate:      base64.StdEncoding.EncodeToString(d.identity.dhPrivate[:]),
		DHPublic:       base64.StdEncoding.EncodeToString(d.identity.dhPublic[:]),
		SignedPrekey: X25519KeyPairState{
			Private: base64.StdEncoding.EncodeToString(d.signedPrekey.Private[:]),
			Public:  base64.StdEncoding.EncodeToString(d.signedPrekey.Public[:]),
		},
		SignedPrekeySig: base64.StdEncoding.EncodeToString(d.signedSig),
		OneTime:         make(map[uint32]X25519KeyPairState, len(d.oneTime)),
		NextOTKID:       d.nextOTKID,
	}
	for id, entry := range d.oneTime {
		state.OneTime[id] = X25519KeyPairState{
			Private: base64.StdEncoding.EncodeToString(entry.key.Private[:]),
			Public:  base64.StdEncoding.EncodeToString(entry.key.Public[:]),
		}
	}
	if len(state.OneTime) == 0 {
		state.OneTime = nil
	}
	return state, nil
}

func ImportDevice(state *DeviceState) (*Device, error) {
	if state == nil {
		return nil, errors.New("cryptocore: nil device state")
	}
	signingPriv, err := base64.StdEncoding.DecodeString(state.SigningPrivate)
	if err != nil {
		return nil, fmt.Errorf("cryptocore: decode signing private: %w", err)
	}
	signingPub, err := base64.StdEncoding.DecodeString(state.SigningPublic)
	if err != nil {
		return nil, fmt.Errorf("cryptocore: decode signing public: %w", err)
	}
	dhPriv, err := decodeFixed(state.DHPrivate, 32)
	if err != nil {
		return nil, fmt.Errorf("cryptocore: decode dh private: %w", err)
	}
	dhPub, err := decodeFixed(state.DHPublic, 32)
	if err != nil {
		return nil, fmt.Errorf("cryptocore: decode dh public: %w", err)
	}
	signedPriv, err := decodeFixed(state.SignedPrekey.Private, 32)
	if err != nil {
		return nil, fmt.Errorf("cryptocore: decode signed prekey private: %w", err)
	}
	signedPub, err := decodeFixed(state.SignedPrekey.Public, 32)
	if err != nil {
		return nil, fmt.Errorf("cryptocore: decode signed prekey public: %w", err)
	}
	sig, err := base64.StdEncoding.DecodeString(state.SignedPrekeySig)
	if err != nil {
		return nil, fmt.Errorf("cryptocore: decode signed prekey sig: %w", err)
	}
	dev := &Device{
		identity: identityKeyPair{
			signingPublic:  append([]byte(nil), signingPub...),
			signingPrivate: append([]byte(nil), signingPriv...),
			dhPrivate:      [32]byte{},
			dhPublic:       [32]byte{},
		},
		signedPrekey: keyPair{},
		signedSig:    append([]byte(nil), sig...),
		oneTime:      make(map[uint32]oneTimeEntry),
		nextOTKID:    state.NextOTKID,
	}
	copy(dev.identity.dhPrivate[:], dhPriv)
	copy(dev.identity.dhPublic[:], dhPub)
	copy(dev.signedPrekey.Private[:], signedPriv)
	copy(dev.signedPrekey.Public[:], signedPub)
	for id, kp := range state.OneTime {
		priv, err := decodeFixed(kp.Private, 32)
		if err != nil {
			return nil, fmt.Errorf("cryptocore: decode one-time private: %w", err)
		}
		pub, err := decodeFixed(kp.Public, 32)
		if err != nil {
			return nil, fmt.Errorf("cryptocore: decode one-time public: %w", err)
		}
		var entry keyPair
		copy(entry.Private[:], priv)
		copy(entry.Public[:], pub)
		dev.oneTime[id] = oneTimeEntry{key: entry}
	}
	return dev, nil
}

func ExportSession(state *SessionState) (*SessionStateSnapshot, error) {
	if state == nil {
		return nil, errors.New("cryptocore: nil session")
	}
	snap := &SessionStateSnapshot{
		RootKey:         base64.StdEncoding.EncodeToString(state.RootKey[:]),
		SendChain:       exportChain(state.SendChain),
		RecvChain:       exportChain(state.RecvChain),
		RatchetPrivate:  base64.StdEncoding.EncodeToString(state.RatchetPrivate[:]),
		RatchetPublic:   base64.StdEncoding.EncodeToString(state.RatchetPublic[:]),
		RemoteRatchet:   base64.StdEncoding.EncodeToString(state.RemoteRatchet[:]),
		RemoteIdentity:  base64.StdEncoding.EncodeToString(state.RemoteIdentity[:]),
		RemoteSignature: base64.StdEncoding.EncodeToString(state.RemoteSignature),
		PN:              state.PN,
		Role:            state.Role,
		PendingPrekey:   state.PendingPrekey,
		Skipped:         make(map[string]string, len(state.skipped)),
	}
	for k, v := range state.skipped {
		snap.Skipped[k] = base64.StdEncoding.EncodeToString(v[:])
	}
	if len(snap.Skipped) == 0 {
		snap.Skipped = nil
	}
	return snap, nil
}

func ImportSession(snapshot *SessionStateSnapshot) (*SessionState, error) {
	if snapshot == nil {
		return nil, errors.New("cryptocore: nil session snapshot")
	}
	rootBytes, err := decodeFixed(snapshot.RootKey, 32)
	if err != nil {
		return nil, fmt.Errorf("cryptocore: decode root key: %w", err)
	}
	send, err := importChain(snapshot.SendChain)
	if err != nil {
		return nil, err
	}
	recv, err := importChain(snapshot.RecvChain)
	if err != nil {
		return nil, err
	}
	ratchetPriv, err := decodeFixed(snapshot.RatchetPrivate, 32)
	if err != nil {
		return nil, fmt.Errorf("cryptocore: decode ratchet private: %w", err)
	}
	ratchetPub, err := decodeFixed(snapshot.RatchetPublic, 32)
	if err != nil {
		return nil, fmt.Errorf("cryptocore: decode ratchet public: %w", err)
	}
	remoteRatchet, err := decodeFixed(snapshot.RemoteRatchet, 32)
	if err != nil {
		return nil, fmt.Errorf("cryptocore: decode remote ratchet: %w", err)
	}
	remoteIdentity, err := decodeFixed(snapshot.RemoteIdentity, 32)
	if err != nil {
		return nil, fmt.Errorf("cryptocore: decode remote identity: %w", err)
	}
	remoteSig, err := base64.StdEncoding.DecodeString(snapshot.RemoteSignature)
	if err != nil {
		return nil, fmt.Errorf("cryptocore: decode remote signature: %w", err)
	}
	var root [32]byte
	copy(root[:], rootBytes)
	sess := &SessionState{
		RootKey:   root,
		SendChain: send,
		RecvChain: recv,
	}
	copy(sess.RatchetPrivate[:], ratchetPriv)
	copy(sess.RatchetPublic[:], ratchetPub)
	copy(sess.RemoteRatchet[:], remoteRatchet)
	copy(sess.RemoteIdentity[:], remoteIdentity)
	sess.RemoteSignature = append([]byte(nil), remoteSig...)
	sess.PN = snapshot.PN
	sess.Role = snapshot.Role
	sess.PendingPrekey = snapshot.PendingPrekey
	sess.skipped = make(map[string][32]byte, len(snapshot.Skipped))
	for k, v := range snapshot.Skipped {
		keyBytes, err := decodeFixed(v, 32)
		if err != nil {
			return nil, fmt.Errorf("cryptocore: decode skipped key: %w", err)
		}
		var key [32]byte
		copy(key[:], keyBytes)
		sess.skipped[k] = key
	}
	return sess, nil
}

func exportChain(cs chainState) ChainStateSnapshot {
	return ChainStateSnapshot{
		Key:   base64.StdEncoding.EncodeToString(cs.Key[:]),
		Index: cs.Index,
	}
}

func importChain(cs ChainStateSnapshot) (chainState, error) {
	keyBytes, err := decodeFixed(cs.Key, 32)
	if err != nil {
		return chainState{}, fmt.Errorf("cryptocore: decode chain key: %w", err)
	}
	var key [32]byte
	copy(key[:], keyBytes)
	return chainState{Key: key, Index: cs.Index}, nil
}

func decodeFixed(in string, size int) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		return nil, err
	}
	if len(data) != size {
		return nil, fmt.Errorf("unexpected length %d, want %d", len(data), size)
	}
	out := make([]byte, size)
	copy(out, data)
	return out, nil
}
