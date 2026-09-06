package service

import (
	"bytes"
	"compress/zlib"
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
// IETF Token Status List draft-21 test helpers
//

// statusValue decodes an entry from the uncompressed byte array used by an
// IETF Token Status List.
//
// Token Status List supports status values with 1, 2, 4, or 8 bits.
// Entries are packed least-significant-bit first.
//
// This is intentionally a test helper. It verifies the representation
// produced by the service without pretending that the production service
// exposes a statusValue function.
func statusValue(bitstring []byte, idx int, bits int) (uint8, error) {
	switch bits {
	case 1, 2, 4, 8:
		// supported
	default:
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

	entriesPerByte := 8 / bits
	entryCount := len(bitstring) * entriesPerByte

	if idx >= entryCount {
		return 0, fmt.Errorf(
			"status index %d out of range",
			idx,
		)
	}

	byteIndex := idx / entriesPerByte
	entryIndex := idx % entriesPerByte
	shift := entryIndex * bits

	var mask uint8

	if bits == 8 {
		mask = 0xff
	} else {
		mask = uint8((1 << bits) - 1)
	}

	return (bitstring[byteIndex] >> shift) & mask, nil
}

//
// IETF Token Status List draft-21 encoding
//

func TestEncodeStatusList_UsesZlibAndBase64URLNoPadding(t *testing.T) {
	bitstring := make([]byte, 16*1024)

	bitstring[0] = 0x80
	bitstring[42] = 0x01

	encoded, err := encodeStatusList(bitstring)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(encoded, "=") {
		t.Fatalf(
			"encoded status list contains base64 padding: %q",
			encoded,
		)
	}

	compressed, err :=
		base64.RawURLEncoding.DecodeString(encoded)

	if err != nil {
		t.Fatalf(
			"status list is not base64url without padding: %v",
			err,
		)
	}

	reader, err :=
		zlib.NewReader(bytes.NewReader(compressed))

	if err != nil {
		t.Fatalf(
			"status list is not zlib encoded: %v",
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
			"decoded status list does not match original bitstring",
		)
	}
}

func TestEncodeStatusList_IsNotGzip(t *testing.T) {
	bitstring := make([]byte, 1024)

	encoded, err := encodeStatusList(bitstring)
	if err != nil {
		t.Fatal(err)
	}

	compressed, err :=
		base64.RawURLEncoding.DecodeString(encoded)

	if err != nil {
		t.Fatal(err)
	}

	if len(compressed) >= 2 &&
		compressed[0] == 0x1f &&
		compressed[1] == 0x8b {

		t.Fatal(
			"status list uses gzip; IETF Token Status List draft-21 requires zlib/DEFLATE",
		)
	}
}

//
// IETF Token Status List draft-21 bit ordering
//

func TestTokenStatusListBitIndex3IsNotSet(t *testing.T) {
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

func TestTokenStatusListBitIndex3IsSet(t *testing.T) {
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

func TestTokenStatusListTwoBitEntriesAreLSBFirst(t *testing.T) {
	//
	// Four 2-bit entries in one byte:
	//
	// idx 0 -> bits 0..1
	// idx 1 -> bits 2..3
	// idx 2 -> bits 4..5
	// idx 3 -> bits 6..7
	//
	// Values:
	//
	// idx0 = 1
	// idx1 = 2
	// idx2 = 3
	// idx3 = 0
	//
	bitstring := []byte{
		0b00111001,
	}

	expected := []uint8{
		1,
		2,
		3,
		0,
	}

	for idx, want := range expected {
		got, err := statusValue(
			bitstring,
			idx,
			2,
		)

		if err != nil {
			t.Fatalf(
				"idx=%d: %v",
				idx,
				err,
			)
		}

		if got != want {
			t.Fatalf(
				"status value at idx=%d = %d, want %d",
				idx,
				got,
				want,
			)
		}
	}
}

func TestTokenStatusListFourBitEntriesAreLSBFirst(t *testing.T) {
	//
	// idx0 = low nibble  = 0x5
	// idx1 = high nibble = 0xA
	//
	bitstring := []byte{
		0xA5,
	}

	first, err := statusValue(
		bitstring,
		0,
		4,
	)

	if err != nil {
		t.Fatal(err)
	}

	if first != 0x05 {
		t.Fatalf(
			"status value at idx=0 = %d, want 5",
			first,
		)
	}

	second, err := statusValue(
		bitstring,
		1,
		4,
	)

	if err != nil {
		t.Fatal(err)
	}

	if second != 0x0A {
		t.Fatalf(
			"status value at idx=1 = %d, want 10",
			second,
		)
	}
}

func TestTokenStatusListEightBitEntries(t *testing.T) {
	bitstring := []byte{
		0x00,
		0x7f,
		0xff,
	}

	expected := []uint8{
		0x00,
		0x7f,
		0xff,
	}

	for idx, want := range expected {
		got, err := statusValue(
			bitstring,
			idx,
			8,
		)

		if err != nil {
			t.Fatalf(
				"idx=%d: %v",
				idx,
				err,
			)
		}

		if got != want {
			t.Fatalf(
				"status value at idx=%d = %d, want %d",
				idx,
				got,
				want,
			)
		}
	}
}

func TestTokenStatusListBitIndexOutOfRange(t *testing.T) {
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

func TestTokenStatusListRejectsUnsupportedBits(t *testing.T) {
	bitstring := make([]byte, 1)

	for _, bits := range []int{
		0,
		3,
		5,
		6,
		7,
		16,
	} {
		_, err := statusValue(
			bitstring,
			0,
			bits,
		)

		if err == nil {
			t.Fatalf(
				"expected unsupported bits error for bits=%d",
				bits,
			)
		}
	}
}

func TestTokenStatusListRejectsNegativeIndex(t *testing.T) {
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
// IETF Token Status List HTTP Accept negotiation
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

func TestResponseEncoderAcceptsEmptyAccept(t *testing.T) {
	rec := httptest.NewRecorder()

	ctx := context.WithValue(
		context.Background(),
		goahttp.AcceptTypeKey,
		"",
	)

	encoder :=
		responseEncoder(ctx, rec)

	if _, ok :=
		encoder.(*statusListJWTEncoder); !ok {

		t.Fatalf(
			"encoder type = %T, want *statusListJWTEncoder for empty Accept",
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
// W3C Bitstring Status List v1.0
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
