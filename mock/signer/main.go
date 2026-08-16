package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/cloudevents/sdk-go/v2/event"
	cloudeventprovider "github.com/eclipse-xfsc/cloud-event-provider"
	messaging "github.com/eclipse-xfsc/nats-message-library"
	"github.com/eclipse-xfsc/nats-message-library/common"
	"github.com/google/uuid"
)

type VerifyRequest struct {
	Credential []byte `json:"credential"`
}

func main() {
	go startHTTP()
	startNATS()
}

func startHTTP() {
	http.HandleFunc("/credential/verify", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"valid": true,
		})
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	log.Println("mock signer http listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func startNATS() {
	natsURL := env("NATS_URL", "nats://nats:4222")
	topic := env("SIGNER_TOPIC", "signer")

	client, err := cloudeventprovider.New(
		cloudeventprovider.Config{
			Protocol: cloudeventprovider.ProtocolTypeNats,
			Settings: cloudeventprovider.NatsConfig{
				Url:          natsURL,
				QueueGroup:   "mock-signer",
				TimeoutInSec: time.Minute,
			},
		},
		cloudeventprovider.ConnectionTypeRep,
		topic,
	)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	log.Printf("mock signer nats listening on %s topic=%s", natsURL, topic)

	if err := client.ReplyCtx(context.Background(), handleSignRequest); err != nil {
		log.Fatal(err)
	}
}

func handleSignRequest(ctx context.Context, ev event.Event) (*event.Event, error) {
	var req messaging.CreateTokenRequest
	if err := json.Unmarshal(ev.Data(), &req); err != nil {
		return nil, err
	}

	reply := messaging.CreateTokenReply{
		Reply: common.Reply{
			TenantId:  req.TenantId,
			RequestId: req.RequestId,
			Error:     nil,
		},
		Token: []byte(fakeJWT(req.Header, req.Payload)),
	}

	data, err := json.Marshal(reply)
	if err != nil {
		return nil, err
	}

	answer, err := cloudeventprovider.NewEvent(
		"mock-signer",
		messaging.SignerServiceSignTokenType,
		data,
	)
	if err != nil {
		return nil, err
	}

	return &answer, nil
}

func fakeJWT(header []byte, payload []byte) string {
	if len(header) == 0 {
		header = []byte(`{"alg":"none","typ":"JWT"}`)
	}

	return base64.RawURLEncoding.EncodeToString(header) +
		"." +
		base64.RawURLEncoding.EncodeToString(payload) +
		"." +
		base64.RawURLEncoding.EncodeToString([]byte("mock-signature-"+uuid.NewString()+"-"+time.Now().UTC().Format(time.RFC3339Nano)))
}

func env(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
