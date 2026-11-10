package identity

import (
	"context"
	"testing"
)

func TestFirebaseVerifierRejectsGarbage(t *testing.T) {
	v, err := NewFirebaseVerifier(context.Background(), "hackathon-10-d0db1", "")
	if err != nil {
		t.Fatalf("init without creds: %v", err)
	}
	if _, err := v.Verify(context.Background(), "not-a-token"); err == nil {
		t.Fatal("garbage token accepted")
	}
}
