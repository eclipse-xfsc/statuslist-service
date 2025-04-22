package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloudevents/sdk-go/v2/event"
	cloudeventprovider "github.com/eclipse-xfsc/cloud-event-provider"
	messaging "github.com/eclipse-xfsc/nats-message-library"
	"github.com/eclipse-xfsc/nats-message-library/common"
	"github.com/eclipse-xfsc/statuslist-service/internal/config"
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

	if strings.Compare(event.Type(), "create") == 0 {

		var eventData messaging.CreateStatusListEntryRequest
		if err := json.Unmarshal(event.Data(), &eventData); err != nil {
			log.Error(err)
			return nil, err
		}

		log.Infof("new Event: %v", eventData)

		if err := db.CreateTableForTenantIdIfNotExists(ctx, eventData.TenantId); err != nil {
			log.Error(err)
			return nil, err
		}

		statusData, err := db.AllocateIndexInCurrentList(ctx, eventData.TenantId)
		if err != nil {
			log.Error(err)
			return nil, err
		}

		var commonError = new(common.Error)

		if err == nil {
			commonError = nil
		} else {
			commonError.Id = uuid.NewString()
			commonError.Status = 400
			commonError.Msg = err.Error()
		}

		var rep = messaging.CreateStatusListEntryReply{
			Reply: common.Reply{
				TenantId:  eventData.TenantId,
				RequestId: eventData.RequestId,
				Error:     commonError,
			},
			Index:     statusData.Index,
			StatusUrl: eventData.Origin + statusData.StatusUrl,
			Purpose:   "revocation",
			Type:      "StatusList2021",
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

	if strings.Compare(event.Type(), "verify") == 0 {
		var eventData messaging.VerifyStatusListEntryRequest
		var err error

		if err = json.Unmarshal(event.Data(), &eventData); err != nil {
			log.Error(err)
			return nil, err
		}

		log.Infof("new Event: %v", eventData)

		var commonError = new(common.Error)

		var request *http.Request
		if eventData.Type == "StatusList2021" {
			r, err := http.NewRequest("GET", eventData.StatusUrl, nil)

			if err != nil {
				log.Error(err)
				return nil, err
			}

			r.Header.Add("Content-Type", "application/vc+ld+json")
			request = r
		}

		res, err := http.DefaultClient.Do(request)

		if err != nil {
			log.Error(err)
			return nil, err
		}

		respBody, err := io.ReadAll(res.Body)

		if err != nil {
			log.Error(err)
			return nil, err
		}

		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			return nil, errors.New("retrieve status list error. result was: " + string(respBody) + " " + res.Status)
		}

		verPayload := VerifyCredentialPayload{Credential: respBody}
		verPayloadBytes, err := json.Marshal(verPayload)
		if err != nil {
			log.Error(err)
			return nil, err
		}

		req, err := http.NewRequest("POST", statusConf.SignerUrl+"/credential/verify", bytes.NewBuffer(verPayloadBytes))
		if err != nil {
			log.Error(err)
			return nil, err
		}
		req.Header.Add("Content-Type", "application/json")
		req.Header.Add("x-namespace", eventData.TenantId)
		req.Header.Add("x-group", eventData.GroupId)
		verRes, err := http.DefaultClient.Do(req)

		if err != nil {
			log.Error(err)
			return nil, err
		}

		verRespBody, err := io.ReadAll(verRes.Body)

		if err != nil {
			log.Error(err)
			return nil, err
		}

		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			return nil, errors.New("signer service call error. result was: " + string(respBody) + " " + res.Status)
		}

		var result map[string]interface{}
		err = json.Unmarshal(verRespBody, &result)

		if err != nil {
			log.Error(err)
			return nil, err
		}

		valid, ok := result["valid"]

		if ok && !valid.(bool) {
			log.Error(err)
			return nil, err
		}

		if !ok {
			log.Error(err)
			return nil, err
		}

		var cred map[string]interface{}
		err = json.Unmarshal(respBody, &cred)

		if err != nil {
			log.Error(err)
			return nil, err
		}

		val, ok := cred["credentialSubject"].(map[string]interface{})

		if !ok {
			log.Error(err)
			return nil, err
		}

		l, ok := val["encodedList"].(string)

		if !ok {
			return nil, err
		}

		l2, err := base64.RawStdEncoding.DecodeString(l)

		if err != nil {
			log.Error(err)
			return nil, err
		}

		reader := bytes.NewReader([]byte(l2))
		gzreader, err := gzip.NewReader(reader)

		if err != nil {
			log.Error(err)
			return nil, err
		}

		blist, err := io.ReadAll(gzreader)

		if err != nil {
			log.Error(err)
			return nil, err
		}

		url, _ := url.Parse(eventData.StatusUrl)

		sha256 := sha256.New()
		cacheId := hex.EncodeToString(sha256.Sum([]byte(url.Host)))

		if err = db.CacheList(ctx, cacheId, blist); err != nil {
			log.Error(err)
			return nil, err
		}

		list := entity.List{
			List: blist,
		}

		if err == nil {
			commonError = nil
		} else {
			commonError.Id = uuid.NewString()
			commonError.Status = 400
			commonError.Msg = err.Error()
		}

		var rep = messaging.VerifyStatusListEntryReply{
			Reply: common.Reply{
				TenantId:  eventData.TenantId,
				RequestId: eventData.RequestId,
				Error:     commonError,
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

	return nil, errors.ErrUnsupported
}

func startMessaging(conf *config.StatusListConfiguration, group *sync.WaitGroup) {
	defer group.Done()
	statusConf = conf
	client, err := cloudeventprovider.New(
		cloudeventprovider.Config{Protocol: cloudeventprovider.ProtocolTypeNats, Settings: conf.Nats},
		cloudeventprovider.ConnectionTypeRep,
		conf.CreationTopic,
	)
	if err != nil {
		panic(err)
	}

	defer client.Close()

	err = client.ReplyCtx(context.Background(), handle)
	if err != nil {
		panic(err)
	}
}

func requestTokenSigning(tenantId, statusList, key, namespace, group, did, host string, bits, listId int) ([]byte, error) {

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

	var list = make(map[string]interface{})

	list["bits"] = bits
	list["lst"] = statusList

	var p = make(map[string]interface{})

	//https://www.ietf.org/archive/id/draft-looker-oauth-jwt-cwt-status-list-01.html#section-4.2
	p["status_list"] = list
	p["iss"] = host
	p["sub"] = host + "/statuslists/" + strconv.Itoa(listId)
	p["iat"] = time.Now().UnixMilli()
	p["exp"] = time.Now().Add(time.Duration(time.Now().Year())).UnixMilli()
	pb, err := json.Marshal(p)

	if err != nil {
		return nil, err
	}

	var ph = make(map[string]interface{})
	ph["kid"] = did + "#" + key
	pbh, err := json.Marshal(ph)

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
		Payload:   pb,
		Header:    pbh,
	}

	b, err := json.Marshal(payload)

	if err != nil {
		return nil, err
	}

	event, err := cloudeventprovider.NewEvent("statuslist-service", messaging.SignerServiceSignTokenType, b)

	if err != nil {
		return nil, err
	}

	rep, err := client.RequestCtx(context.Background(), event)

	if err != nil {
		return nil, err
	}

	var tok messaging.CreateTokenReply

	err = json.Unmarshal(rep.Data(), &tok)

	return tok.Token, err

}
