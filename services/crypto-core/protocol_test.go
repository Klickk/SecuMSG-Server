package cryptocore

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func deterministicReader(size int) *bytes.Reader {
	buf := make([]byte, size)
	for i := range buf {
		buf[i] = byte(i % 251)
	}
	return bytes.NewReader(buf)
}

func TestX3DHDoubleRatchetDeterministic(t *testing.T) {
	restore := UseDeterministicRandom(deterministicReader(4096))
	defer restore()

	alice, err := GenerateIdentityKeypair()
	if err != nil {
		t.Fatalf("alice identity: %v", err)
	}
	bob, err := GenerateIdentityKeypair()
	if err != nil {
		t.Fatalf("bob identity: %v", err)
	}
	bundle, err := bob.PublishPrekeyBundle(2)
	if err != nil {
		t.Fatalf("bundle: %v", err)
	}
	aliceSess, handshake, err := alice.InitSession(bundle)
	if err != nil {
		t.Fatalf("init session: %v", err)
	}
	if got := hex.EncodeToString(handshake.EphemeralKey[:]); got != "dc2cca31e8e43bbd91dff7e475cca3347eb478107d5bd765aba4ae4a30c35d44" {
		t.Fatalf("unexpected handshake key: %s", got)
	}
	if got := hex.EncodeToString(aliceSess.RootKey[:]); got != "599a9d4b42e82e9f389c697aea3847e8b9385bd27bbe72b9ef28ca17838f2142" {
		t.Fatalf("unexpected root key: %s", got)
	}
	if got := hex.EncodeToString(aliceSess.SendChain.Key[:]); got != "b9db519a2fa4409f769a615c6a8342a63f315cd389ae4e0416044f811fee967c" {
		t.Fatalf("unexpected send chain key: %s", got)
	}
	bobSess, err := bob.AcceptSession(handshake)
	if err != nil {
		t.Fatalf("accept session: %v", err)
	}

	msg := []byte("hello bob")
	ct, header, err := Encrypt(aliceSess, msg)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if got := hex.EncodeToString(ct); got != "a8105aa6824cac0cbd41ded989db0d528ae5011a00bb0e238b" {
		t.Fatalf("unexpected ciphertext: %s", got)
	}
	plaintext, err := Decrypt(bobSess, ct, header)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(plaintext, msg) {
		t.Fatalf("decrypt mismatch: got %q want %q", plaintext, msg)
	}

	reply := []byte("hi alice")
	ct2, header2, err := Encrypt(bobSess, reply)
	if err != nil {
		t.Fatalf("encrypt reply: %v", err)
	}
	plaintext2, err := Decrypt(aliceSess, ct2, header2)
	if err != nil {
		t.Fatalf("decrypt reply: %v", err)
	}
	if !bytes.Equal(plaintext2, reply) {
		t.Fatalf("reply mismatch: got %q want %q", plaintext2, reply)
	}
}
