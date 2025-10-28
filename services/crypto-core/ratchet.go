package cryptocore

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

const (
	hkdfInfoRatchet       = "SecuMSG-DR"
	maxSkippedMessageKeys = 64
)

// Encrypt derives the next sending message key, returns the ciphertext and the
// message header that must accompany the ciphertext.
func Encrypt(session *SessionState, plaintext []byte) ([]byte, *MessageHeader, error) {
	if session == nil {
		return nil, nil, errors.New("cryptocore: nil session")
	}
	if isZeroKey(session.SendChain.Key) {
		if err := RotateRatchetOnSend(session, nil); err != nil {
			return nil, nil, err
		}
	}
	newCK, mk := kdfChain(session.SendChain.Key)
	n := session.SendChain.Index
	session.SendChain.Key = newCK
	session.SendChain.Index++

	key, nonce, err := deriveCipherParams(mk)
	if err != nil {
		return nil, nil, err
	}
	aead, err := chacha20poly1305.New(key[:])
	if err != nil {
		return nil, nil, err
	}
	header := &MessageHeader{DHPublic: session.RatchetPublic, PN: session.PN, N: n, Nonce: nonce}
	ad := header.associatedData()
	ciphertext := aead.Seal(nil, nonce[:], plaintext, ad)
	return ciphertext, header, nil
}

// Decrypt attempts to open the ciphertext using the provided header, handling
// skipped message keys as necessary.
func Decrypt(session *SessionState, ciphertext []byte, header *MessageHeader) ([]byte, error) {
	if session == nil {
		return nil, errors.New("cryptocore: nil session")
	}
	if header == nil {
		return nil, errors.New("cryptocore: nil header")
	}
	if mk, ok := session.consumeSkipped(header); ok {
		key, nonce, err := deriveCipherParams(mk)
		if err != nil {
			return nil, err
		}
		aead, err := chacha20poly1305.New(key[:])
		if err != nil {
			return nil, err
		}
		plaintext, err := aead.Open(nil, nonce[:], ciphertext, header.associatedData())
		if err != nil {
			return nil, ErrDecryptionFailed
		}
		return plaintext, nil
	}
	if err := RotateRatchetOnRecv(session, header); err != nil {
		return nil, err
	}
	if header.N < session.RecvChain.Index {
		return nil, ErrDuplicateMessage
	}
	for session.RecvChain.Index < header.N {
		newCK, mk := kdfChain(session.RecvChain.Key)
		session.storeSkippedKey(session.RemoteRatchet, session.RecvChain.Index, mk)
		session.RecvChain.Key = newCK
		session.RecvChain.Index++
	}
	newCK, mk := kdfChain(session.RecvChain.Key)
	session.RecvChain.Key = newCK
	session.RecvChain.Index++
	key, nonce, err := deriveCipherParams(mk)
	if err != nil {
		return nil, err
	}
	aead, err := chacha20poly1305.New(key[:])
	if err != nil {
		return nil, err
	}
	plaintext, err := aead.Open(nil, nonce[:], ciphertext, header.associatedData())
	if err != nil {
		return nil, ErrDecryptionFailed
	}
	return plaintext, nil
}

// RotateRatchetOnSend advances the local sending ratchet and prepares a fresh
// sending chain based on a newly generated DH key pair.
func RotateRatchetOnSend(session *SessionState, _ *MessageHeader) error {
	if session == nil {
		return errors.New("cryptocore: nil session")
	}
	if isZeroKey(session.RemoteRatchet) {
		return ErrInvalidRemoteKey
	}
	kp, err := generateX25519KeyPair()
	if err != nil {
		return err
	}
	dh, err := curve25519.X25519(kp.Private[:], session.RemoteRatchet[:])
	if err != nil {
		return err
	}
	root, send, err := kdfRoot(session.RootKey[:], dh)
	if err != nil {
		return err
	}
	session.RootKey = root
	session.PN = session.SendChain.Index
	session.SendChain = chainState{Key: send, Index: 0}
	session.RatchetPrivate = kp.Private
	session.RatchetPublic = kp.Public
	return nil
}

// RotateRatchetOnRecv processes a header received from the remote party and
// updates the receiving chain if a new remote ratchet key is presented.
func RotateRatchetOnRecv(session *SessionState, header *MessageHeader) error {
	if session == nil {
		return errors.New("cryptocore: nil session")
	}
	if header == nil {
		return errors.New("cryptocore: nil header")
	}
	if header.DHPublic == session.RemoteRatchet {
		return nil
	}
	dh, err := curve25519.X25519(session.RatchetPrivate[:], header.DHPublic[:])
	if err != nil {
		return err
	}
	root, recv, err := kdfRoot(session.RootKey[:], dh)
	if err != nil {
		return err
	}
	session.RootKey = root
	session.RemoteRatchet = header.DHPublic
	session.RecvChain = chainState{Key: recv, Index: 0}
	session.SendChain.Key = [32]byte{}
	session.SendChain.Index = 0
	session.PN = header.PN
	return nil
}

func kdfRoot(root, dh []byte) ([32]byte, [32]byte, error) {
	hk := hkdf.New(sha256.New, dh, root, []byte(hkdfInfoRatchet))
	var newRoot, chain [32]byte
	if _, err := io.ReadFull(hk, newRoot[:]); err != nil {
		return [32]byte{}, [32]byte{}, err
	}
	if _, err := io.ReadFull(hk, chain[:]); err != nil {
		return [32]byte{}, [32]byte{}, err
	}
	return newRoot, chain, nil
}

func kdfChain(chain [32]byte) ([32]byte, [32]byte) {
	mk := hmacSHA256(chain[:], []byte{0x01})
	out := hmacSHA256(chain[:], []byte{0x02})
	var next, msg [32]byte
	copy(next[:], mk)
	copy(msg[:], out)
	return next, msg
}

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

func deriveCipherParams(mk [32]byte) ([32]byte, [12]byte, error) {
	hk := hkdf.New(sha256.New, mk[:], nil, []byte("SecuMSG-AEAD"))
	var key [32]byte
	var nonce [12]byte
	if _, err := io.ReadFull(hk, key[:]); err != nil {
		return [32]byte{}, [12]byte{}, err
	}
	if _, err := io.ReadFull(hk, nonce[:]); err != nil {
		return [32]byte{}, [12]byte{}, err
	}
	return key, nonce, nil
}

func (h *MessageHeader) associatedData() []byte {
	buf := make([]byte, 32+4+4)
	copy(buf, h.DHPublic[:])
	binary.BigEndian.PutUint32(buf[32:], h.PN)
	binary.BigEndian.PutUint32(buf[36:], h.N)
	return buf
}

func isZeroKey(k [32]byte) bool {
	var zero [32]byte
	return k == zero
}

func (s *SessionState) storeSkippedKey(pub [32]byte, index uint32, key [32]byte) {
	if s.skipped == nil {
		s.skipped = make(map[string][32]byte)
	}
	if len(s.skipped) >= maxSkippedMessageKeys {
		for k := range s.skipped {
			delete(s.skipped, k)
			break
		}
	}
	name := skippedKey(pub, index)
	s.skipped[name] = key
}

func (s *SessionState) consumeSkipped(header *MessageHeader) ([32]byte, bool) {
	if s.skipped == nil {
		return [32]byte{}, false
	}
	name := skippedKey(header.DHPublic, header.N)
	if val, ok := s.skipped[name]; ok {
		delete(s.skipped, name)
		return val, true
	}
	return [32]byte{}, false
}

func skippedKey(pub [32]byte, index uint32) string {
	buf := make([]byte, 32+4)
	copy(buf, pub[:])
	binary.BigEndian.PutUint32(buf[32:], index)
	return string(buf)
}
