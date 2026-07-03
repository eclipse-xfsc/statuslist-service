package database

import (
	"context"
	"time"

	pgPkg "github.com/eclipse-xfsc/microservice-core-go/pkg/db/postgres"
	"github.com/eclipse-xfsc/statuslist-service/internal/entity"
)

type AllocateStatusListEntryRequest struct {
	TenantID       string
	Origin         string
	Key            string
	DID            string
	Namespace      string
	Group          string
	Type           string
	Purpose        string
	ExpirationDate *time.Time
}

type StatusListWithSigner struct {
	TenantID          string
	ListID            int
	Type              string
	Version           int
	Bitstring         []byte
	DID               string
	KeyRef            string
	Namespace         string
	Group             string
	Origin            string
	Purpose           string
	StatusURL         string
	MaxExpirationDate *time.Time
}

type DbConnection interface {
	AllocateIndexInCurrentList(ctx context.Context, req AllocateStatusListEntryRequest) (*entity.StatusData, error)
	RevokeCredentialInSpecifiedList(ctx context.Context, tenantId string, listId int, index int) error
	GetStatusList(ctx context.Context, tenantId string, listId int) ([]byte, error)
	GetStatusListWithSigner(ctx context.Context, tenantId string, listId int) (*StatusListWithSigner, error)
	CacheList(ctx context.Context, cacheId string, list []byte) error
	Ping() bool
	Close()
}

type Database struct {
	DbConnection
}

func New(ctx context.Context, config pgPkg.Config, listSizeInBytes int) (*Database, error) {
	dbConnection, err := newPostgresConnection(config, ctx, listSizeInBytes)
	return &Database{DbConnection: dbConnection}, err
}
