package cryptocore

import (
	"crypto/ed25519"
)

type SessionRole int

const (
	RoleInitiator SessionRole = iota
	RoleResponder
)

type Device struct {
	identity     identityKeyPair
	signedPrekey keyPair
	signedSig    []byte
	oneTime      map[uint32]oneTimeEntry
	nextOTKID    uint32
}

type identityKeyPair struct {
	signingPublic  ed25519.PublicKey
	signingPrivate ed25519.PrivateKey
	dhPrivate      [32]byte
	dhPublic       [32]byte
}

type keyPair struct {
	Private [32]byte
	Public  [32]byte
}

type oneTimeEntry struct {
	key keyPair
}

type PrekeyBundle struct {
	IdentityKey          [32]byte
	IdentitySignatureKey []byte
	SignedPrekey         [32]byte
	SignedPrekeySig      []byte
	OneTimePrekeys       []OneTimePrekey
}

type OneTimePrekey struct {
	ID     uint32
	Public [32]byte
}

type HandshakeMessage struct {
	IdentityKey          [32]byte
	IdentitySignatureKey []byte
	EphemeralKey         [32]byte
	OneTimePrekeyID      *uint32
}

type chainState struct {
	Key   [32]byte
	Index uint32
}

type SessionState struct {
	RootKey         [32]byte
	SendChain       chainState
	RecvChain       chainState
	RatchetPrivate  [32]byte
	RatchetPublic   [32]byte
	RemoteRatchet   [32]byte
	RemoteIdentity  [32]byte
	RemoteSignature []byte
	PN              uint32
	Role            SessionRole
	PendingPrekey   *uint32
	skipped         map[string][32]byte
}

type MessageHeader struct {
	DHPublic [32]byte
	PN       uint32
	N        uint32
	Nonce    [12]byte
}
