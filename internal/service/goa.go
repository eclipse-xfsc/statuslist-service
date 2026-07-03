package service

import (
	"fmt"
	"net/http"
	"sync"

	goahttp "goa.design/goa/v3/http"

	statusserver "github.com/eclipse-xfsc/statuslist-service/gen/http/status/server"
	status "github.com/eclipse-xfsc/statuslist-service/gen/status"
	"github.com/eclipse-xfsc/statuslist-service/internal/config"
	"github.com/eclipse-xfsc/statuslist-service/internal/database"
)

func StartGoa(conf *config.StatusListConfiguration, group *sync.WaitGroup, db *database.Database) {
	defer group.Done()

	svc := NewStatusService(db)
	endpoints := status.NewEndpoints(svc)

	mux := goahttp.NewMuxer()

	server := statusserver.New(
		endpoints,
		mux,
		goahttp.RequestDecoder,
		goahttp.ResponseEncoder,
		nil,
		nil,
	)

	statusserver.Mount(mux, server)

	addr := fmt.Sprintf(":%d", conf.ListenPort)

	if err := http.ListenAndServe(addr, mux); err != nil {
		panic(err)
	}
}
