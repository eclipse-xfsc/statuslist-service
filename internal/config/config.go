package config

import (
	"fmt"

	"github.com/eclipse-xfsc/microservice-core-go/pkg/logr"

	cloudeventprovider "github.com/eclipse-xfsc/cloud-event-provider"
	"github.com/eclipse-xfsc/microservice-core-go/pkg/config"
	pgPkg "github.com/eclipse-xfsc/microservice-core-go/pkg/db/postgres"
	"github.com/kelseyhightower/envconfig"
)

var logger logr.Logger

type StatusListConfiguration struct {
	config.BaseConfig `mapstructure:",squash"`
	Database          pgPkg.Config                  `mapstructure:"database" envconfig:"DATABASE"`
	CreationTopic     string                        `mapstructure:"creationTopic" envconfig:"CREATIONTOPIC" default:"status.data.create"`
	ListSizeInBytes   int                           `mapstructure:"listSizeInBytes" envconfig:"LISTSIZEINBYTES" default:"1024"`
	Nats              cloudeventprovider.NatsConfig `envconfig:"NATS"`
	SignerTopic       string                        `envconfig:"SIGNER_TOPIC" default:"signer"`
	SignerUrl         string                        `envconfig:"SIGNER_URL" default:"signer"`
	DefaultKey        string                        `envconfig:"DEFAULT_KEY" default:"test"`
	DefaultDid        string                        `envconfig:"DEFAULT_DID" default:"did:web:localhost:8081:v1:did:document"`
	DefaultNamespace  string                        `envconfig:"DEFAULT_NAMESPACE" default:"transit"`
	DefaultGroup      string                        `envconfig:"DEFAULT_GROUP" default:""`
	DefaultHost       string                        `envconfig:"DEFAULT_HOST" default:"http://localhost:8081/v1/tenants/transit"`
	DefaultListType   string                        `envconfig:"DEFAULT_LISTTYPE" default:"StatusList2021"`
}

var CurrentStatusListConfig StatusListConfiguration

func Load() error {
	err := envconfig.Process("STATUSLIST", &CurrentStatusListConfig)
	if err != nil {
		panic(fmt.Sprintf("failed to load config from env: %+v", err))
	}

	return err
}

func SetLogger(log logr.Logger) {
	logger = log
}

func GetLogger() logr.Logger {
	return logger
}
