package service

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	goahttp "goa.design/goa/v3/http"

	statusserver "github.com/eclipse-xfsc/statuslist-service/gen/http/status/server"
	status "github.com/eclipse-xfsc/statuslist-service/gen/status"
	"github.com/eclipse-xfsc/statuslist-service/internal/config"
	"github.com/eclipse-xfsc/statuslist-service/internal/database"
)

type statusListJWTEncoder struct {
	w http.ResponseWriter
}

func (e *statusListJWTEncoder) Encode(v interface{}) error {
	var token string

	switch value := v.(type) {
	case string:
		token = value

	case []byte:
		token = string(value)

	default:
		return fmt.Errorf(
			"status list JWT encoder: unsupported type %T",
			v,
		)
	}

	e.w.Header().Set(
		"Content-Type",
		"application/statuslist+jwt",
	)

	_, err := e.w.Write([]byte(token))
	return err
}

func responseEncoder(
	ctx context.Context,
	w http.ResponseWriter,
) goahttp.Encoder {

	accept, _ := ctx.Value(goahttp.AcceptTypeKey).(string)

	if accept == "application/statuslist+jwt" {
		return &statusListJWTEncoder{
			w: w,
		}
	}

	return goahttp.ResponseEncoder(ctx, w)
}

func StartGoa(conf *config.StatusListConfiguration, group *sync.WaitGroup, db *database.Database) {
	defer group.Done()

	svc := NewStatusService(db)
	endpoints := status.NewEndpoints(svc)

	mux := goahttp.NewMuxer()

	server := statusserver.New(
		endpoints,
		mux,
		goahttp.RequestDecoder,
		responseEncoder,
		nil,
		nil,
	)

	statusserver.Mount(mux, server)

	addr := fmt.Sprintf(":%d", conf.ListenPort)

	if err := http.ListenAndServe(addr, mux); err != nil {
		panic(err)
	}
}
