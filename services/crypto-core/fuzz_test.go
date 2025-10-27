package cryptocore

import (
	"bytes"
	"testing"
)

func FuzzHeaderCounters(f *testing.F) {
	f.Add(uint32(0), uint32(0), []byte("payload"))
	f.Add(uint32(5), uint32(1), []byte{})
	f.Fuzz(func(t *testing.T, n, pn uint32, payload []byte) {
		restore := UseDeterministicRandom(bytes.NewReader(bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 512)))
		defer restore()

		alice, err := GenerateIdentityKeypair()
		if err != nil {
			t.Fatalf("alice identity: %v", err)
		}
		bob, err := GenerateIdentityKeypair()
		if err != nil {
			t.Fatalf("bob identity: %v", err)
		}
		bundle, err := bob.PublishPrekeyBundle(4)
		if err != nil {
			t.Fatalf("bundle: %v", err)
		}
		aliceSess, handshake, err := alice.InitSession(bundle)
		if err != nil {
			t.Fatalf("init: %v", err)
		}
		bobSess, err := bob.AcceptSession(handshake)
		if err != nil {
			t.Fatalf("accept: %v", err)
		}

		ct, header, err := Encrypt(aliceSess, []byte("seed"))
		if err != nil {
			t.Fatalf("seed encrypt: %v", err)
		}
		if _, err := Decrypt(bobSess, ct, header); err != nil {
			_ = err
		}

		ct2, header2, err := Encrypt(aliceSess, payload)
		if err != nil {
			t.Fatalf("encrypt payload: %v", err)
		}
		header2.N = n % 128
		header2.PN = pn % 64
		if header2.N%2 == 0 {
			header2.DHPublic[0] ^= 0x01
		}
		_, _ = Decrypt(bobSess, ct2, header2)
	})
}
