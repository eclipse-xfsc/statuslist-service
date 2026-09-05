package service

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	goahttp "goa.design/goa/v3/http"

	"github.com/eclipse-xfsc/statuslist-service/internal/database"
)

//
// Test helper for reading a 1-bit OAuth Status List entry.
//
// Keep this here only as a test utility. It verifies the expected
// least-significant-bit-first representation without pretending that
// the service already exposes a statusValue function.
//

func statusValue(bitstring []byte, idx int, bits int) (uint8, error) {
	if bits != 1 {
		return 0, fmt.Errorf(
			"unsupported bits value: %d",
			bits,
		)
	}

	if idx < 0 {
		return 0, fmt.Errorf(
			"invalid status index: %d",
			idx,
		)
	}

	byteIndex := idx / 8
	bitIndex := idx % 8

	if byteIndex >= len(bitstring) {
		return 0, fmt.Errorf(
			"status index %d out of range",
			idx,
		)
	}

	return (bitstring[byteIndex] >> bitIndex) & 1, nil
}

//
// Existing Status List encoding
//

func TestEncodeStatusList_UsesGzipAndBase64URLNoPadding(t *testing.T) {
	bitstring := make([]byte, 16*1024)

	bitstring[0] = 0x80
	bitstring[42] = 0x01

	encoded, err := encodeStatusList(bitstring)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(encoded, "=") {
		t.Fatalf(
			"encoded list contains base64 padding: %q",
			encoded,
		)
	}

	compressed, err :=
		base64.RawURLEncoding.DecodeString(encoded)

	if err != nil {
		t.Fatalf(
			"encoded list is not base64url without padding: %v",
			err,
		)
	}

	reader, err :=
		gzip.NewReader(bytes.NewReader(compressed))

	if err != nil {
		t.Fatalf(
			"encoded list is not gzip data: %v",
			err,
		)
	}

	defer reader.Close()

	decoded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(decoded, bitstring) {
		t.Fatal(
			"decoded bitstring does not match original",
		)
	}
}

//
// OAuth Status List bit ordering
//

func TestOAuthStatusListBitIndex3IsNotSet(t *testing.T) {
	bitstring := make([]byte, 16)

	got, err := statusValue(
		bitstring,
		3,
		1,
	)

	if err != nil {
		t.Fatal(err)
	}

	if got != 0 {
		t.Fatalf(
			"status value at idx=3 = %d, want 0",
			got,
		)
	}
}

func TestOAuthStatusListBitIndex3IsSet(t *testing.T) {
	bitstring := make([]byte, 16)

	//
	// 1-bit status entries use least-significant-bit-first packing.
	//
	// idx=3:
	//
	// byteIndex = 3 / 8 = 0
	// bitIndex  = 3 %% 8 = 3
	//

	bitstring[0] |= 1 << 3

	got, err := statusValue(
		bitstring,
		3,
		1,
	)

	if err != nil {
		t.Fatal(err)
	}

	if got != 1 {
		t.Fatalf(
			"status value at idx=3 = %d, want 1",
			got,
		)
	}
}

func TestOAuthStatusListBitIndexOutOfRange(t *testing.T) {
	bitstring := make([]byte, 1)

	_, err := statusValue(
		bitstring,
		8,
		1,
	)

	if err == nil {
		t.Fatal(
			"expected out-of-range error",
		)
	}
}

func TestOAuthStatusListRejectsUnsupportedBits(t *testing.T) {
	bitstring := make([]byte, 1)

	_, err := statusValue(
		bitstring,
		0,
		2,
	)

	if err == nil {
		t.Fatal(
			"expected unsupported bits error",
		)
	}
}

func TestOAuthStatusListRejectsNegativeIndex(t *testing.T) {
	bitstring := make([]byte, 1)

	_, err := statusValue(
		bitstring,
		-1,
		1,
	)

	if err == nil {
		t.Fatal(
			"expected negative index error",
		)
	}
}

//
// HTTP application/statuslist+jwt encoder
//

func TestStatusListJWTEncoderWritesRawCompactJWT(t *testing.T) {
	rec := httptest.NewRecorder()

	encoder := &statusListJWTEncoder{
		w: rec,
	}

	token :=
		"eyJhbGciOiJFUzI1NiJ9.eyJzdWIiOiJ0ZXN0In0.signature"

	if err := encoder.Encode(token); err != nil {
		t.Fatal(err)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/statuslist+jwt" {
		t.Fatalf(
			"Content-Type = %q, want application/statuslist+jwt",
			got,
		)
	}

	if got := rec.Body.String(); got != token {
		t.Fatalf(
			"body = %q, want raw compact JWT",
			got,
		)
	}

	if strings.HasPrefix(
		rec.Body.String(),
		`"`,
	) {
		t.Fatal(
			"JWT was unexpectedly JSON quoted",
		)
	}
}

func TestStatusListJWTEncoderAcceptsBytes(t *testing.T) {
	rec := httptest.NewRecorder()

	encoder := &statusListJWTEncoder{
		w: rec,
	}

	token :=
		[]byte("eyJ.header.payload.signature")

	if err := encoder.Encode(token); err != nil {
		t.Fatal(err)
	}

	if got := rec.Body.String(); got != string(token) {
		t.Fatalf(
			"body = %q, want %q",
			got,
			string(token),
		)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/statuslist+jwt" {
		t.Fatalf(
			"Content-Type = %q, want application/statuslist+jwt",
			got,
		)
	}
}

func TestStatusListJWTEncoderRejectsUnsupportedType(t *testing.T) {
	rec := httptest.NewRecorder()

	encoder := &statusListJWTEncoder{
		w: rec,
	}

	err := encoder.Encode(
		struct {
			Token string
		}{
			Token: "abc",
		},
	)

	if err == nil {
		t.Fatal(
			"expected unsupported type error",
		)
	}

	if !strings.Contains(
		err.Error(),
		"unsupported type",
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}

//
// HTTP Accept negotiation
//

func TestResponseEncoderUsesStatusListJWTEncoderForExactAccept(t *testing.T) {
	rec := httptest.NewRecorder()

	ctx := context.WithValue(
		context.Background(),
		goahttp.AcceptTypeKey,
		"application/statuslist+jwt",
	)

	encoder :=
		responseEncoder(ctx, rec)

	if _, ok :=
		encoder.(*statusListJWTEncoder); !ok {

		t.Fatalf(
			"encoder type = %T, want *statusListJWTEncoder",
			encoder,
		)
	}
}

func TestResponseEncoderDoesNotUseJWTEncoderForJSON(t *testing.T) {
	rec := httptest.NewRecorder()

	ctx := context.WithValue(
		context.Background(),
		goahttp.AcceptTypeKey,
		"application/json",
	)

	encoder :=
		responseEncoder(ctx, rec)

	if _, ok :=
		encoder.(*statusListJWTEncoder); ok {

		t.Fatal(
			"unexpected statusListJWTEncoder for application/json",
		)
	}
}

func TestResponseEncoderAcceptsWildcardAccept(t *testing.T) {
	rec := httptest.NewRecorder()

	ctx := context.WithValue(
		context.Background(),
		goahttp.AcceptTypeKey,
		"*/*",
	)

	encoder :=
		responseEncoder(ctx, rec)

	if _, ok :=
		encoder.(*statusListJWTEncoder); !ok {

		t.Fatalf(
			"encoder type = %T, want *statusListJWTEncoder for wildcard Accept",
			encoder,
		)
	}
}

func TestResponseEncoderAcceptsCombinedMediaTypes(t *testing.T) {
	rec := httptest.NewRecorder()

	ctx := context.WithValue(
		context.Background(),
		goahttp.AcceptTypeKey,
		"application/statuslist+jwt, application/json;q=0.5",
	)

	encoder :=
		responseEncoder(ctx, rec)

	if _, ok :=
		encoder.(*statusListJWTEncoder); !ok {

		t.Fatalf(
			"encoder type = %T, want *statusListJWTEncoder",
			encoder,
		)
	}
}

func TestResponseEncoderAcceptsStatusListJWTWithParameters(t *testing.T) {
	rec := httptest.NewRecorder()

	ctx := context.WithValue(
		context.Background(),
		goahttp.AcceptTypeKey,
		"application/statuslist+jwt;q=1.0",
	)

	encoder :=
		responseEncoder(ctx, rec)

	if _, ok :=
		encoder.(*statusListJWTEncoder); !ok {

		t.Fatalf(
			"encoder type = %T, want *statusListJWTEncoder",
			encoder,
		)
	}
}

//
// W3C Bitstring Status List
//

func TestBuildBitstringStatusListCredential_ConformsToVCBitstringStatusList(t *testing.T) {
	list :=
		testStatusList(
			"BitstringStatusListCredential",
		)

	credential :=
		buildCredential(list, "abc")

	subject :=
		credential["credentialSubject"].(map[string]any)

	assertContains(
		t,
		credential["@context"].([]string),
		"https://www.w3.org/ns/credentials/v2",
	)

	assertContains(
		t,
		credential["@context"].([]string),
		"https://www.w3.org/ns/credentials/status/v1",
	)

	assertContains(
		t,
		credential["type"].([]string),
		"VerifiableCredential",
	)

	assertContains(
		t,
		credential["type"].([]string),
		"BitstringStatusListCredential",
	)

	if credential["issuer"] != list.DID {
		t.Fatalf(
			"issuer = %v",
			credential["issuer"],
		)
	}

	if _, ok :=
		credential["validFrom"].(string); !ok {

		t.Fatal(
			"validFrom missing",
		)
	}

	if subject["type"] != "BitstringStatusList" {
		t.Fatalf(
			"credentialSubject.type = %v",
			subject["type"],
		)
	}

	if subject["statusPurpose"] != "revocation" {
		t.Fatalf(
			"statusPurpose = %v",
			subject["statusPurpose"],
		)
	}

	if subject["encodedList"] != "abc" {
		t.Fatalf(
			"encodedList = %v",
			subject["encodedList"],
		)
	}
}

//
// StatusList2021 legacy
//

func TestBuildStatusList2021Credential_ConformsToStatusList2021Shape(t *testing.T) {
	list :=
		testStatusList(
			"StatusList2021",
		)

	credential :=
		buildCredential(list, "abc")

	subject :=
		credential["credentialSubject"].(map[string]any)

	assertContains(
		t,
		credential["@context"].([]string),
		"https://www.w3.org/2018/credentials/v1",
	)

	assertContains(
		t,
		credential["@context"].([]string),
		"https://w3id.org/vc/status-list/2021/v1",
	)

	assertContains(
		t,
		credential["type"].([]string),
		"VerifiableCredential",
	)

	assertContains(
		t,
		credential["type"].([]string),
		"StatusList2021Credential",
	)

	if credential["issuer"] != list.DID {
		t.Fatalf(
			"issuer = %v",
			credential["issuer"],
		)
	}

	if _, ok :=
		credential["issuanceDate"].(string); !ok {

		t.Fatal(
			"issuanceDate missing",
		)
	}

	if subject["type"] != "StatusList2021" {
		t.Fatalf(
			"credentialSubject.type = %v",
			subject["type"],
		)
	}

	if subject["statusPurpose"] != "revocation" {
		t.Fatalf(
			"statusPurpose = %v",
			subject["statusPurpose"],
		)
	}

	if subject["encodedList"] != "abc" {
		t.Fatalf(
			"encodedList = %v",
			subject["encodedList"],
		)
	}
}

func TestBuildCredential_DefaultsToStatusList2021(t *testing.T) {
	list :=
		testStatusList("")

	credential :=
		buildCredential(list, "abc")

	subject :=
		credential["credentialSubject"].(map[string]any)

	assertContains(
		t,
		credential["type"].([]string),
		"StatusList2021Credential",
	)

	if subject["type"] != "StatusList2021" {
		t.Fatalf(
			"credentialSubject.type = %v",
			subject["type"],
		)
	}
}

//
// Shared fixtures
//

func testStatusList(
	statusType string,
) *database.StatusListWithSigner {

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

func assertContains(
	t *testing.T,
	values []string,
	want string,
) {
	t.Helper()

	for _, value := range values {
		if value == want {
			return
		}
	}

	t.Fatalf(
		"%q not found in %v",
		want,
		values,
	)
}
