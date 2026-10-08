package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func TestBuildCanonicalMessage(t *testing.T) {
	params := map[string]string{
		"timestamp":  "1700000000",
		"company_id": "123456",
		"sign":       "ignore_this",
	}
	body := `{"foo":"bar"}`
	msg := BuildCanonicalMessage(params, body)
	expected := `company_id=123456&timestamp=1700000000&body={"foo":"bar"}`
	if msg != expected {
		t.Fatalf("expected '%s', got '%s'", expected, msg)
	}
}

func TestGenerateFlicknovelSign(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	privBase64 := base64.StdEncoding.EncodeToString(priv.Seed())

	params := map[string]string{
		"company_id": "355549587538358272",
		"timestamp":  "1700000000",
	}
	body := "{}"

	sig, err := GenerateFlicknovelSign(params, body, privBase64)
	if err != nil {
		t.Fatalf("generate sign failed: %v", err)
	}
	if sig == "" {
		t.Fatal("signature is empty")
	}
}
