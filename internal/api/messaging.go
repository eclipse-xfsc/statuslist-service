package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/cloudevents/sdk-go/v2/event"
	cloudeventprovider "github.com/eclipse-xfsc/cloud-event-provider"
	messaging "github.com/eclipse-xfsc/nats-message-library"
	"github.com/eclipse-xfsc/nats-message-library/common"
	"github.com/eclipse-xfsc/statuslist-service/internal/config"
	"github.com/eclipse-xfsc/statuslist-service/internal/database"
	"github.com/eclipse-xfsc/statuslist-service/internal/entity"
	"github.com/google/uuid"
	"github.com/klauspost/compress/gzip"
	log "github.com/sirupsen/logrus"
)

var statusConf *config.StatusListConfiguration

type VerifyCredentialPayload struct {
	Credential []byte `json:"credential"`
}

func handle(ctx context.Context, event event.Event) (*event.Event, error) {

	switch event.Type() {
	case messaging.TopicStatusData:
		return handleCreate(ctx, event)
	case messaging.TopicStatusDataVerify:
		return handleVerify(ctx, event)
	default:
		return nil, fmt.Errorf("unsupported event type: %s", event.Type())
	}

}

func handleCreate(ctx context.Context, event event.Event) (*event.Event, error) {
	var eventData messaging.CreateStatusListEntryRequest
	if err := json.Unmarshal(event.Data(), &eventData); err != nil {
		log.Error(err)
		return nil, err
	}

	log.Infof("new create event: %v", eventData)

	statusData, err := db.AllocateIndexInCurrentList(ctx, database.AllocateStatusListEntryRequest{
		TenantID:       eventData.TenantId,
		Origin:         eventData.Origin,
		Key:            eventData.Key,
		DID:            eventData.DID,
		Namespace:      eventData.Namespace,
		Group:          eventData.Group,
		Type:           eventData.Type,
		Purpose:        eventData.Purpose,
		ExpirationDate: &eventData.ExpirationDate,
	})

	commonError := buildCommonError(err)
	if err != nil {
		log.Error(err)
	}

	rep := messaging.CreateStatusListEntryReply{
		Reply: common.Reply{
			TenantId:  eventData.TenantId,
			RequestId: eventData.RequestId,
			Error:     commonError,
		},
		Index:     statusData.Index,
		StatusUrl: eventData.Origin + statusData.StatusUrl,
		Purpose:   purposeOrDefault(eventData.Purpose),
		Type:      typeOrDefault(eventData.Type),
	}

	answerData, err := json.Marshal(rep)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	answerEvent, err := cloudeventprovider.NewEvent("status-list-service", messaging.EventTypeStatus, answerData)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	return &answerEvent, nil
}

func handleVerify(ctx context.Context, event event.Event) (*event.Event, error) {
	var eventData messaging.VerifyStatusListEntryRequest
	if err := json.Unmarshal(event.Data(), &eventData); err != nil {
		log.Error(err)
		return nil, err
	}

	log.Infof("new verify event: %v", eventData)

	request, err := http.NewRequest(http.MethodGet, eventData.StatusUrl, nil)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	if eventData.Type == "StatusList2021" || eventData.Type == "" {
		request.Header.Add("Content-Type", "application/vc+ld+json")
	}

	res, err := http.DefaultClient.Do(request)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	defer res.Body.Close()

	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, errors.New("retrieve status list error. result was: " + string(respBody) + " " + res.Status)
	}

	verPayload := VerifyCredentialPayload{Credential: respBody}
	verPayloadBytes, err := json.Marshal(verPayload)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, statusConf.SignerUrl+"/credential/verify", bytes.NewBuffer(verPayloadBytes))
	if err != nil {
		log.Error(err)
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("x-namespace", eventData.Namespace)
	req.Header.Add("x-group", eventData.Group)

	verRes, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	defer verRes.Body.Close()

	verRespBody, err := io.ReadAll(verRes.Body)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	if verRes.StatusCode != http.StatusOK {
		return nil, errors.New("signer service call error. result was: " + string(verRespBody) + " " + verRes.Status)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(verRespBody, &result); err != nil {
		log.Error(err)
		return nil, err
	}

	valid, ok := result["valid"].(bool)
	if !ok || !valid {
		return nil, errors.New("credential verification failed")
	}

	var cred map[string]interface{}
	if err := json.Unmarshal(respBody, &cred); err != nil {
		log.Error(err)
		return nil, err
	}

	credentialSubject, ok := cred["credentialSubject"].(map[string]interface{})
	if !ok {
		return nil, errors.New("credentialSubject not found")
	}

	encodedList, ok := credentialSubject["encodedList"].(string)
	if !ok {
		return nil, errors.New("encodedList not found")
	}

	compressedList, err := base64.RawStdEncoding.DecodeString(encodedList)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	reader := bytes.NewReader(compressedList)
	gzreader, err := gzip.NewReader(reader)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	defer gzreader.Close()

	bitstring, err := io.ReadAll(gzreader)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	parsedURL, err := url.Parse(eventData.StatusUrl)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	hasher := sha256.New()
	hasher.Write([]byte(parsedURL.Host))
	cacheID := hex.EncodeToString(hasher.Sum(nil))

	if err := db.CacheList(ctx, cacheID, bitstring); err != nil {
		log.Error(err)
		return nil, err
	}

	list := entity.List{
		List: bitstring,
	}

	rep := messaging.VerifyStatusListEntryReply{
		Reply: common.Reply{
			TenantId:  eventData.TenantId,
			RequestId: eventData.RequestId,
			Error:     nil,
		},
		Revocated: list.CheckBitAtIndex(eventData.Index),
		Suspended: false,
	}

	answerData, err := json.Marshal(rep)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	answerEvent, err := cloudeventprovider.NewEvent("status-list-service", messaging.EventTypeStatus, answerData)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	return &answerEvent, nil
}

func startMessaging(conf *config.StatusListConfiguration, group *sync.WaitGroup) {
	defer group.Done()

	statusConf = conf

	client, err := cloudeventprovider.New(
		cloudeventprovider.Config{
			Protocol: cloudeventprovider.ProtocolTypeNats,
			Settings: conf.Nats,
		},
		cloudeventprovider.ConnectionTypeRep,
		conf.CreationTopic,
	)
	if err != nil {
		panic(err)
	}
	defer client.Close()

	if err := client.ReplyCtx(context.Background(), handle); err != nil {
		panic(err)
	}
}

func requestTokenSigning(tenantId, statusList, key, namespace, group, did, origin string, bits, listId int) ([]byte, error) {
	client, err := cloudeventprovider.New(cloudeventprovider.Config{
		Protocol: cloudeventprovider.ProtocolTypeNats,
		Settings: cloudeventprovider.NatsConfig{
			Url:          config.CurrentStatusListConfig.Nats.Url,
			QueueGroup:   config.CurrentStatusListConfig.Nats.QueueGroup,
			TimeoutInSec: config.CurrentStatusListConfig.Nats.TimeoutInSec,
		},
	}, cloudeventprovider.Req, config.CurrentStatusListConfig.SignerTopic)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	list := map[string]interface{}{
		"bits": bits,
		"lst":  statusList,
	}

	payloadMap := map[string]interface{}{
		"status_list": list,
		"iss":         origin,
		"sub":         origin + "/statuslists/" + strconv.Itoa(listId),
		"iat":         time.Now().Unix(),
		"exp":         time.Now().AddDate(1, 0, 0).Unix(),
	}

	payloadBytes, err := json.Marshal(payloadMap)
	if err != nil {
		return nil, err
	}

	headerMap := map[string]interface{}{
		"kid": did + "#" + key,
	}

	headerBytes, err := json.Marshal(headerMap)
	if err != nil {
		return nil, err
	}

	payload := messaging.CreateTokenRequest{
		Request: common.Request{
			TenantId:  tenantId,
			RequestId: uuid.NewString(),
		},
		Namespace: namespace,
		Group:     group,
		Key:       key,
		Payload:   payloadBytes,
		Header:    headerBytes,
	}

	requestBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	signEvent, err := cloudeventprovider.NewEvent("statuslist-service", messaging.SignerServiceSignTokenType, requestBytes)
	if err != nil {
		return nil, err
	}

	replyEvent, err := client.RequestCtx(context.Background(), signEvent)
	if err != nil {
		return nil, err
	}

	var tokenReply messaging.CreateTokenReply
	if err := json.Unmarshal(replyEvent.Data(), &tokenReply); err != nil {
		return nil, err
	}

	return tokenReply.Token, nil
}

func buildCommonError(err error) *common.Error {
	if err == nil {
		return nil
	}

	return &common.Error{
		Id:     uuid.NewString(),
		Status: 400,
		Msg:    err.Error(),
	}
}

func typeOrDefault(value string) string {
	if value == "" {
		return "StatusList2021"
	}
	return value
}

func purposeOrDefault(value string) string {
	if value == "" {
		return "revocation"
	}
	return value
}
