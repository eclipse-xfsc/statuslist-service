package database

import (
	"context"

	pgPkg "github.com/eclipse-xfsc/microservice-core-go/pkg/db/postgres"
	"github.com/eclipse-xfsc/statuslist-service/internal/entity"
)

type DbConnection interface {
	AllocateIndexInCurrentList(ctx context.Context, tenantId string) (*entity.StatusData, error)
	RevokeCredentialInSpecifiedList(ctx context.Context, tenantId string, listId int, index int) error
	CreateTableForTenantIdIfNotExists(ctx context.Context, tenantId string) error
	GetStatusList(ctx context.Context, tenantId string, listId int) ([]byte, error)
	CacheList(ctx context.Context, cacheId string, list []byte) error
	Ping() bool
	Close()
}

// TablePrefix is needed for table name cause of postgres name convention -> no integers allowed
const TablePrefix = "tenant_id_"

type Database struct {
	DbConnection
}

func New(ctx context.Context, config pgPkg.Config, listSizeInBytes int) (*Database, error) {
	dbConnection, err := newPostgresConnection(config, ctx, listSizeInBytes)
	return &Database{DbConnection: dbConnection}, err
}
