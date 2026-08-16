package service

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
	"testing"

	"github.com/eclipse-xfsc/statuslist-service/internal/database"
)

func TestEncodeStatusList_UsesGzipAndBase64URLNoPadding(t *testing.T) {
	bitstring := make([]byte, 16*1024)
	bitstring[0] = 0x80
	bitstring[42] = 0x01

	encoded, err := encodeStatusList(bitstring)
	if err != nil {
		t.Fatal(err)
	}

	compressed, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("encodedList is not base64url without padding: %v", err)
	}

	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("encodedList is not gzip data: %v", err)
	}
	defer reader.Close()

	decoded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(decoded, bitstring) {
		t.Fatal("decoded bitstring does not match original")
	}
}

func TestBuildBitstringStatusListCredential_ConformsToVCBitstringStatusList(t *testing.T) {
	list := testStatusList("BitstringStatusListCredential")

	credential := buildCredential(list, "abc")
	subject := credential["credentialSubject"].(map[string]any)

	assertContains(t, credential["@context"].([]string), "https://www.w3.org/ns/credentials/v2")
	assertContains(t, credential["@context"].([]string), "https://www.w3.org/ns/credentials/status/v1")
	assertContains(t, credential["type"].([]string), "VerifiableCredential")
	assertContains(t, credential["type"].([]string), "BitstringStatusListCredential")

	if credential["issuer"] != list.DID {
		t.Fatalf("issuer = %v", credential["issuer"])
	}
	if _, ok := credential["validFrom"].(string); !ok {
		t.Fatal("validFrom missing")
	}

	if subject["type"] != "BitstringStatusList" {
		t.Fatalf("credentialSubject.type = %v", subject["type"])
	}
	if subject["statusPurpose"] != "revocation" {
		t.Fatalf("statusPurpose = %v", subject["statusPurpose"])
	}
	if subject["encodedList"] != "abc" {
		t.Fatalf("encodedList = %v", subject["encodedList"])
	}
}

func TestBuildStatusList2021Credential_ConformsToStatusList2021Shape(t *testing.T) {
	list := testStatusList("StatusList2021")

	credential := buildCredential(list, "abc")
	subject := credential["credentialSubject"].(map[string]any)

	assertContains(t, credential["@context"].([]string), "https://www.w3.org/2018/credentials/v1")
	assertContains(t, credential["@context"].([]string), "https://w3id.org/vc/status-list/2021/v1")
	assertContains(t, credential["type"].([]string), "VerifiableCredential")
	assertContains(t, credential["type"].([]string), "StatusList2021Credential")

	if credential["issuer"] != list.DID {
		t.Fatalf("issuer = %v", credential["issuer"])
	}
	if _, ok := credential["issuanceDate"].(string); !ok {
		t.Fatal("issuanceDate missing")
	}

	if subject["type"] != "StatusList2021" {
		t.Fatalf("credentialSubject.type = %v", subject["type"])
	}
	if subject["statusPurpose"] != "revocation" {
		t.Fatalf("statusPurpose = %v", subject["statusPurpose"])
	}
	if subject["encodedList"] != "abc" {
		t.Fatalf("encodedList = %v", subject["encodedList"])
	}
}

func TestBuildCredential_DefaultsToStatusList2021(t *testing.T) {
	list := testStatusList("")

	credential := buildCredential(list, "abc")
	subject := credential["credentialSubject"].(map[string]any)

	assertContains(t, credential["type"].([]string), "StatusList2021Credential")

	if subject["type"] != "StatusList2021" {
		t.Fatalf("credentialSubject.type = %v", subject["type"])
	}
}

func testStatusList(statusType string) *database.StatusListWithSigner {
	return &database.StatusListWithSigner{
		TenantID:  "tenant-a",
		ListID:    8,
		Type:      statusType,
		Version:   1,
		DID:       "did:web:issuer.example",
		KeyRef:    "key-1",
		Namespace: "issuer-a",
		Group:     "group-a",
		Origin:    "https://issuer.example",
		Purpose:   "revocation",
		StatusURL: "/status/tenant-a/8",
		Bitstring: make([]byte, 16*1024),
	}
}

func assertContains(t *testing.T, values []string, want string) {
	t.Helper()

	for _, value := range values {
		if value == want {
			return
		}
	}

	t.Fatalf("%q not found in %v", want, values)
}
