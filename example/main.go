package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	cloudeventprovider "github.com/eclipse-xfsc/cloud-event-provider"
	messaging "github.com/eclipse-xfsc/nats-message-library"
	"github.com/eclipse-xfsc/nats-message-library/common"
)

func main() {
	client, _ := cloudeventprovider.New(
		cloudeventprovider.Config{Protocol: cloudeventprovider.ProtocolTypeNats, Settings: cloudeventprovider.NatsConfig{
			Url:          "nats://localhost:4222",
			TimeoutInSec: time.Minute,
		}},
		cloudeventprovider.ConnectionTypeReq,
		messaging.TopicStatusData,
	)

	reader := bufio.NewReader(os.Stdin)
	for {
		var req = messaging.CreateStatusListEntryRequest{
			Request: common.Request{TenantId: "123"},
			Origin:  "https://testtesttest",
		}

		b, _ := json.Marshal(req)

		testEvent, _ := cloudeventprovider.NewEvent("test-status", messaging.EventTypeStatus, b)

		ev, _ := client.RequestCtx(context.Background(), testEvent)

		var rep messaging.CreateStatusListEntryReply

		json.Unmarshal(ev.Data(), &rep)

		fmt.Println(rep)
		reader.ReadString('\n')
	}
}
