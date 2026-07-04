package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	status "github.com/eclipse-xfsc/statuslist-service/gen/status"
	"github.com/eclipse-xfsc/statuslist-service/internal/database"
	"github.com/eclipse-xfsc/statuslist-service/internal/signer"
	"github.com/klauspost/compress/gzip"
)

const (
	typeStatusList2021Credential      = "StatusList2021Credential"
	typeStatusList2021                = "StatusList2021"
	typeBitstringStatusListCredential = "BitstringStatusListCredential"
	typeBitstringStatusList           = "BitstringStatusList"

	defaultPurpose = "revocation"
	defaultBits    = 1
)

type StatusService struct {
	db database.DbConnection
}

func ptrString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func NewStatusService(db database.DbConnection) *StatusService {
	return &StatusService{db: db}
}

var _ status.Service = (*StatusService)(nil)

func (s *StatusService) Health(ctx context.Context) (string, error) {
	if !s.db.Ping() {
		return "", fmt.Errorf("database ping failed")
	}

	return "ok", nil
}

func (s *StatusService) Revoke(ctx context.Context, p *status.RevokePayload) (*status.RevokeResult, error) {
	if err := s.db.RevokeCredentialInSpecifiedList(ctx, p.TenantID, p.ListID, p.Index); err != nil {
		return nil, err
	}

	return &status.RevokeResult{
		TenantID: p.TenantID,
		ListID:   p.ListID,
		Index:    p.Index,
		Status:   "revoked",
	}, nil
}

func (s *StatusService) GetList(ctx context.Context, p *status.GetListPayload) (any, error) {
	statusList, err := s.db.GetStatusListWithSigner(ctx, p.TenantID, p.ListID)
	if err != nil {
		return nil, err
	}

	encodedList, err := encodeStatusList(statusList.Bitstring)
	if err != nil {
		return nil, err
	}

	content := ptrString(p.Accept)

	switch content {
	case "application/statuslist+jwt":
		token, err := signer.RequestTokenSigning(
			p.TenantID,
			encodedList,
			statusList.KeyRef,
			statusList.Namespace,
			statusList.Group,
			statusList.DID,
			statusList.Origin,
			defaultBits,
			p.ListID,
		)
		if err != nil {
			return nil, err
		}

		return string(token), nil

	case "application/vc+ld+json":
		return buildCredential(statusList, encodedList), nil

	default:
		return map[string]any{
			"tenantId": p.TenantID,
			"listId":   p.ListID,
			"type":     normalizedListType(statusList.Type),
			"purpose":  purposeOrDefault(statusList.Purpose),
			"list":     encodedList,
		}, nil
	}
}

func encodeStatusList(bitstring []byte) (string, error) {
	var buf bytes.Buffer

	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write(bitstring); err != nil {
		return "", err
	}

	if err := writer.Close(); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buf.Bytes()), nil
}

func buildCredential(list *database.StatusListWithSigner, encodedList string) map[string]any {
	switch normalizedListType(list.Type) {
	case typeBitstringStatusListCredential, typeBitstringStatusList:
		return buildBitstringStatusListCredential(list, encodedList)

	case typeStatusList2021Credential, typeStatusList2021:
		fallthrough

	default:
		return buildStatusList2021Credential(list, encodedList)
	}
}

func buildBitstringStatusListCredential(list *database.StatusListWithSigner, encodedList string) map[string]any {
	now := time.Now().UTC().Format(time.RFC3339)
	id := statusListURL(list)
	subjectID := id + "#list"

	return map[string]any{
		"@context": []string{
			"https://www.w3.org/ns/credentials/v2",
			"https://www.w3.org/ns/credentials/status/v1",
		},
		"id":        id,
		"type":      []string{"VerifiableCredential", typeBitstringStatusListCredential},
		"issuer":    list.DID,
		"validFrom": now,
		"credentialSubject": map[string]any{
			"id":            subjectID,
			"type":          typeBitstringStatusList,
			"statusPurpose": purposeOrDefault(list.Purpose),
			"encodedList":   encodedList,
		},
	}
}

func buildStatusList2021Credential(list *database.StatusListWithSigner, encodedList string) map[string]any {
	now := time.Now().UTC().Format(time.RFC3339)
	id := statusListURL(list)
	subjectID := id + "#list"

	return map[string]any{
		"@context": []string{
			"https://www.w3.org/2018/credentials/v1",
			"https://w3id.org/vc/status-list/2021/v1",
		},
		"id":           id,
		"type":         []string{"VerifiableCredential", typeStatusList2021Credential},
		"issuer":       list.DID,
		"issuanceDate": now,
		"credentialSubject": map[string]any{
			"id":            subjectID,
			"type":          typeStatusList2021,
			"statusPurpose": purposeOrDefault(list.Purpose),
			"encodedList":   encodedList,
		},
	}
}

func preferredContentType(accept string) string {
	value := strings.TrimSpace(accept)

	value = strings.ToLower(value)

	if strings.Contains(value, "application/vc+ld+json") {
		return "vc+ld+json"
	}

	if strings.Contains(value, "application/statuslist+jwt") {
		return "statuslist+jwt"
	}

	if strings.Contains(value, "application/json") {
		return "json"
	}

	return value
}

func normalizedListType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "bitstringstatuslistcredential":
		return typeBitstringStatusListCredential
	case "bitstringstatuslist":
		return typeBitstringStatusList
	case "statuslist2021credential":
		return typeStatusList2021Credential
	case "statuslist2021", "":
		return typeStatusList2021
	default:
		return value
	}
}

func purposeOrDefault(value string) string {
	if strings.TrimSpace(value) == "" {
		return defaultPurpose
	}
	return value
}

func statusListURL(list *database.StatusListWithSigner) string {
	if list.StatusURL != "" {
		return joinOriginAndPath(list.Origin, list.StatusURL)
	}

	return fmt.Sprintf("%s/status/%s/%d", strings.TrimRight(list.Origin, "/"), list.TenantID, list.ListID)
}

func joinOriginAndPath(origin string, path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}

	return strings.TrimRight(origin, "/") + "/" + strings.TrimLeft(path, "/")
}
