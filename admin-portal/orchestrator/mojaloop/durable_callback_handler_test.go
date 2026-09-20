package mojaloop

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/http/httptest"
	"testing"
)

func TestVerifyJWSSignature(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DFSP_PUBLIC_KEY_TESTDFSP", string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})))
	t.Setenv("MOJALOOP_ENV", "production")
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","kid":"test-key"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"transferId":"transfer-1"}`))
	digest := sha256.Sum256([]byte(header + "." + payload))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("PUT", "/transfers/transfer-1", nil)
	request.Header.Set("FSPIOP-Source", "testdfsp")
	request.Header.Set("FSPIOP-Signature", header+"."+payload+"."+base64.RawURLEncoding.EncodeToString(signature))
	handler := &DurableCallbackHandler{}
	if !handler.verifyJWSSignature(request) {
		t.Fatal("valid signature was rejected")
	}
	request.Header.Set("FSPIOP-Signature", header+"."+payload+".invalid")
	if handler.verifyJWSSignature(request) {
		t.Fatal("invalid signature was accepted")
	}
}

func TestUnsignedCallbackRejectedByDefault(t *testing.T) {
	t.Setenv("MOJALOOP_ENV", "development")
	t.Setenv("MOJALOOP_ALLOW_UNSIGNED", "")
	if (&DurableCallbackHandler{}).verifyJWSSignature(httptest.NewRequest("PUT", "/transfers/transfer-1", nil)) {
		t.Fatal("unsigned callback was accepted without explicit override")
	}
}
