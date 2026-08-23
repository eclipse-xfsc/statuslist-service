package signer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	cloudeventprovider "github.com/eclipse-xfsc/cloud-event-provider"
	messaging "github.com/eclipse-xfsc/nats-message-library"
	"github.com/eclipse-xfsc/nats-message-library/common"
	"github.com/eclipse-xfsc/statuslist-service/internal/config"
	"github.com/google/uuid"
)

func RequestTokenSigning(
	tenantID string,
	groupid string,
	statusList string,
	key string,
	namespace string,
	group string,
	did string,
	origin string,
	bits int,
	listID int,
) ([]byte, error) {

	client, err := cloudeventprovider.New(
		cloudeventprovider.Config{
			Protocol: cloudeventprovider.ProtocolTypeNats,
			Settings: cloudeventprovider.NatsConfig{
				Url:          config.CurrentStatusListConfig.Nats.Url,
				QueueGroup:   config.CurrentStatusListConfig.Nats.QueueGroup,
				TimeoutInSec: config.CurrentStatusListConfig.Nats.TimeoutInSec,
			},
		},
		cloudeventprovider.Req,
		config.CurrentStatusListConfig.SignerTopic,
	)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	payload := map[string]any{
		"status_list": map[string]any{
			"bits": bits,
			"lst":  statusList,
		},
		"iss": origin,
		"sub": fmt.Sprintf("%s/status/%s/%d", strings.TrimRight(origin, "/"), tenantID, listID),
		"iat": time.Now().Unix(),
		"exp": time.Now().AddDate(1, 0, 0).Unix(),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	header := map[string]any{
		"kid": did + "#" + key,
	}

	headerBytes, err := json.Marshal(header)
	if err != nil {
		return nil, err
	}

	req := messaging.CreateTokenRequest{
		Request: common.Request{
			TenantId:  tenantID,
			GroupId:   groupid,
			RequestId: uuid.NewString(),
		},
		Namespace: namespace,
		Group:     group,
		Key:       key,
		Payload:   payloadBytes,
		Header:    headerBytes,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	ev, err := cloudeventprovider.NewEvent(
		"statuslist-service",
		messaging.SignerServiceSignTokenType,
		body,
	)
	if err != nil {
		return nil, err
	}

	reply, err := client.RequestCtx(context.Background(), ev)
	if err != nil {
		return nil, err
	}

	var tokenReply messaging.CreateTokenReply
	if err := json.Unmarshal(reply.Data(), &tokenReply); err != nil {
		return nil, err
	}

	if tokenReply.Error != nil {
		return nil, fmt.Errorf(tokenReply.Error.Msg)
	}

	return tokenReply.Token, nil
}
