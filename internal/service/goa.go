package service

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"strings"
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

	if acceptsStatusListJWT(accept) {
		w.Header().Set(
			"Content-Type",
			"application/statuslist+jwt",
		)

		return &statusListJWTEncoder{
			w: w,
		}
	}

	return goahttp.ResponseEncoder(ctx, w)
}

func acceptsStatusListJWT(accept string) bool {
	accept = strings.TrimSpace(accept)

	//
	// Wallet interoperability:
	//
	// Empty Accept and */* mean that the client accepts any
	// representation. Since this endpoint returns a Status List JWT,
	// prefer application/statuslist+jwt.
	//
	if accept == "" || accept == "*/*" {
		return true
	}

	for _, part := range strings.Split(accept, ",") {
		part = strings.TrimSpace(part)

		mediaType, _, err := mime.ParseMediaType(part)
		if err != nil {
			continue
		}

		if mediaType == "application/statuslist+jwt" {
			return true
		}
	}

	return false
}

func StartGoa(
	conf *config.StatusListConfiguration,
	group *sync.WaitGroup,
	db *database.Database,
) {
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

	addr := fmt.Sprintf(
		":%d",
		conf.ListenPort,
	)

	if err := http.ListenAndServe(
		addr,
		mux,
	); err != nil {
		panic(err)
	}
}
