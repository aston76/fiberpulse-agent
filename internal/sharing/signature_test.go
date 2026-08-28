package sharing

import "testing"

func TestSignatureBindsBodyAndSequence(t *testing.T) {
	id, err := NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	sig, err := id.Sign("POST", "/v1/batches", "2026-08-28T00:00:00Z", "nonce", 7, []byte("body"))
	if err != nil {
		t.Fatal(err)
	}
	if !Verify(id.Public, sig, "POST", "/v1/batches", "2026-08-28T00:00:00Z", "nonce", 7, []byte("body")) {
		t.Fatal("valid signature rejected")
	}
	if Verify(id.Public, sig, "POST", "/v1/batches", "2026-08-28T00:00:00Z", "nonce", 8, []byte("body")) {
		t.Fatal("sequence replay accepted")
	}
	if Verify(id.Public, sig, "POST", "/v1/batches", "2026-08-28T00:00:00Z", "nonce", 7, []byte("changed")) {
		t.Fatal("changed body accepted")
	}
}
