package api

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/klauspost/compress/gzip"
)

func TestDecodeTokenStatusListUsesZlib(t *testing.T) {
	want := []byte{0x00, 0x01, 0x02, 0x04, 0x08}

	var compressed bytes.Buffer
	zw := zlib.NewWriter(&compressed)
	if _, err := zw.Write(want); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	payload, err := json.Marshal(map[string]any{
		"status_list": map[string]any{
			"bits": 1,
			"lst":  base64.RawURLEncoding.EncodeToString(compressed.Bytes()),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	token := base64.RawURLEncoding.EncodeToString([]byte(`{"typ":"statuslist+jwt"}`)) + "." +
		base64.RawURLEncoding.EncodeToString(payload) + ".signature"

	got, err := decodeVerifiedStatusList([]byte(token), "application/statuslist+jwt")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("decoded list = %v, want %v", got, want)
	}
}

func TestDecodeCredentialStatusListKeepsGzip(t *testing.T) {
	want := []byte{0x00, 0x01, 0x02, 0x04, 0x08}

	var compressed bytes.Buffer
	gw := gzip.NewWriter(&compressed)
	if _, err := gw.Write(want); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}

	credential, err := json.Marshal(map[string]any{
		"credentialSubject": map[string]any{
			"encodedList": base64.RawURLEncoding.EncodeToString(compressed.Bytes()),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := decodeVerifiedStatusList(credential, "application/vc+ld+json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("decoded list = %v, want %v", got, want)
	}
}

func TestVerifyStatusListAccept(t *testing.T) {
	tests := map[string]string{
		"TokenStatusList":           "application/statuslist+jwt",
		"TokenStatusListCredential": "application/statuslist+jwt",
		"JWTStatusList":             "application/statuslist+jwt",
		"statuslist+jwt":            "application/statuslist+jwt",
		"StatusList2021":            "application/vc+ld+json",
		"BitstringStatusList":       "application/vc+ld+json",
		"":                          "application/vc+ld+json",
	}

	for input, want := range tests {
		if got := verifyStatusListAccept(input); got != want {
			t.Fatalf("verifyStatusListAccept(%q) = %q, want %q", input, got, want)
		}
	}
}
