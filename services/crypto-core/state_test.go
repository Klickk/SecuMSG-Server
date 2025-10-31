package cryptocore

import "testing"

func TestDeviceExportImport(t *testing.T) {
	dev, err := GenerateIdentityKeypair()
	if err != nil {
		t.Fatalf("GenerateIdentityKeypair: %v", err)
	}
	if _, err := dev.PublishPrekeyBundle(2); err != nil {
		t.Fatalf("PublishPrekeyBundle: %v", err)
	}
	state, err := dev.Export()
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	restored, err := ImportDevice(state)
	if err != nil {
		t.Fatalf("ImportDevice: %v", err)
	}
	if got, want := restored.nextOTKID, dev.nextOTKID; got != want {
		t.Fatalf("nextOTKID mismatch: got %d want %d", got, want)
	}
	if len(restored.oneTime) != len(dev.oneTime) {
		t.Fatalf("one-time map length mismatch: got %d want %d", len(restored.oneTime), len(dev.oneTime))
	}
}

func TestSessionExportImport(t *testing.T) {
	alice, err := GenerateIdentityKeypair()
	if err != nil {
		t.Fatalf("GenerateIdentityKeypair(alice): %v", err)
	}
	bob, err := GenerateIdentityKeypair()
	if err != nil {
		t.Fatalf("GenerateIdentityKeypair(bob): %v", err)
	}
	bundle, err := bob.PublishPrekeyBundle(0)
	if err != nil {
		t.Fatalf("bob PublishPrekeyBundle: %v", err)
	}
	aliceSess, hs, err := alice.InitSession(bundle)
	if err != nil {
		t.Fatalf("InitSession: %v", err)
	}
	bobSess, err := bob.AcceptSession(hs)
	if err != nil {
		t.Fatalf("AcceptSession: %v", err)
	}
	if _, _, err := Encrypt(aliceSess, []byte("hi")); err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	snap, err := ExportSession(aliceSess)
	if err != nil {
		t.Fatalf("ExportSession: %v", err)
	}
	restored, err := ImportSession(snap)
	if err != nil {
		t.Fatalf("ImportSession: %v", err)
	}
	ct, header, err := Encrypt(restored, []byte("hello"))
	if err != nil {
		t.Fatalf("Encrypt(restored): %v", err)
	}
	if _, err := Decrypt(bobSess, ct, header); err != nil {
		t.Fatalf("Decrypt(bob): %v", err)
	}
}
