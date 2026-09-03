package database

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	ctxPkg "github.com/eclipse-xfsc/microservice-core-go/pkg/ctx"
	pgPkg "github.com/eclipse-xfsc/microservice-core-go/pkg/db/postgres"
	errPkg "github.com/eclipse-xfsc/microservice-core-go/pkg/err"
	"github.com/eclipse-xfsc/statuslist-service/internal/entity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations

var Migrations embed.FS

const (
	defaultStatusListType = "StatusList2021"
	defaultPurpose        = "revocation"
)

type postgresConnection struct {
	conn            *pgxpool.Pool
	listSizeInBytes int
}

func newPostgresConnection(database pgPkg.Config, ctx context.Context, listSizeInBytes int) (DbConnection, error) {
	logger := ctxPkg.GetLogger(ctx)

	errChan := make(chan error)
	go errPkg.LogChan(logger, errChan)

	conn, err := pgPkg.ConnectRetry(ctx, database, time.Minute, errChan)
	if err != nil {
		logger.Error(err, "failed to connect to postgres")
		os.Exit(1)
	}

	if err := pgPkg.MigrateUP(conn, Migrations, "migrations"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("run postgres migrations: %w", err)
	}

	return &postgresConnection{
		conn:            conn,
		listSizeInBytes: listSizeInBytes,
	}, nil
}

func (pc *postgresConnection) Ping() bool {
	return pc.conn.Ping(context.Background()) == nil
}

func (pc *postgresConnection) AllocateIndexInCurrentList(ctx context.Context, req AllocateStatusListEntryRequest) (*entity.StatusData, error) {
	tx, err := pc.conn.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.ReadCommitted,
		AccessMode: pgx.ReadWrite,
	})
	if err != nil {
		return nil, fmt.Errorf("could not start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	statusType := req.Type
	if statusType == "" {
		statusType = defaultStatusListType
	}

	purpose := req.Purpose
	if purpose == "" {
		purpose = defaultPurpose
	}

	row := tx.QueryRow(ctx, `
		SELECT list_id, bitstring, capacity, next_index
		FROM status_lists
		WHERE tenant_id = $1
		  AND origin = $2
		  AND type = $3
		  AND purpose = $4
		  AND did = $5
		  AND key_ref = $6
		  AND namespace = $7
		  AND COALESCE("group", '') = COALESCE($8, '')
		  AND next_index < capacity
		ORDER BY list_id ASC
		FOR UPDATE
		LIMIT 1
	`,
		req.TenantID,
		req.Origin,
		statusType,
		purpose,
		req.DID,
		req.Key,
		req.Namespace,
		req.Group,
	)

	var (
		listID    int
		bitstring []byte
		capacity  int
		nextIndex int
	)

	err = row.Scan(&listID, &bitstring, &capacity, &nextIndex)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("select current status list: %w", err)
	}

	if errors.Is(err, pgx.ErrNoRows) {
		newList := entity.NewList(pc.listSizeInBytes)

		index, err := newList.AllocateNextFreeIndex()
		if err != nil {
			return nil, fmt.Errorf("allocate index in new list: %w", err)
		}

		err = tx.QueryRow(ctx, `
			INSERT INTO status_lists (
				tenant_id,
				type,
				version,
				capacity,
				next_index,
				bitstring,
				did,
				key_ref,
				namespace,
				"group",
				origin,
				purpose,
				max_expiration_date
			)
			VALUES ($1, $2, 1, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			RETURNING list_id
		`,
			req.TenantID,
			statusType,
			pc.listSizeInBytes*8,
			index+1,
			newList.List,
			req.DID,
			req.Key,
			req.Namespace,
			req.Group,
			req.Origin,
			purpose,
			req.ExpirationDate,
		).Scan(&listID)
		if err != nil {
			return nil, fmt.Errorf("insert new status list: %w", err)
		}

		statusURL := req.TenantID + "/" + strconv.Itoa(listID)

		_, err = tx.Exec(ctx, `
			UPDATE status_lists
			SET status_url = $1,
			    updated_at = now()
			WHERE tenant_id = $2
			  AND list_id = $3
		`, statusURL, req.TenantID, listID)
		if err != nil {
			return nil, fmt.Errorf("update status url: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit transaction: %w", err)
		}

		return entity.NewStatusData(req.Origin, index, listID), nil
	}

	currentList := entity.List{
		ListId: listID,
		List:   bitstring,
		Free:   capacity - nextIndex,
	}

	index, err := currentList.AllocateNextFreeIndex()
	if err != nil {
		return nil, fmt.Errorf("allocate index in current list: %w", err)
	}

	_, err = tx.Exec(ctx, `
		UPDATE status_lists
		SET bitstring = $1,
		    next_index = $2,
		    max_expiration_date = CASE
		        WHEN $3::timestamptz IS NULL THEN max_expiration_date
		        WHEN max_expiration_date IS NULL THEN $3
		        ELSE GREATEST(max_expiration_date, $3)
		    END,
		    updated_at = now()
		WHERE tenant_id = $4
		  AND list_id = $5
	`,
		currentList.List,
		index+1,
		req.ExpirationDate,
		req.TenantID,
		listID,
	)
	if err != nil {
		return nil, fmt.Errorf("update current status list: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return entity.NewStatusData(req.Origin, index, listID), nil
}

func (pc *postgresConnection) GetStatusList(ctx context.Context, tenantId string, listId int) ([]byte, error) {
	var bitstring []byte

	err := pc.conn.QueryRow(ctx, `
		SELECT bitstring
		FROM status_lists
		WHERE tenant_id = $1
		  AND list_id = $2
		LIMIT 1
	`, tenantId, listId).Scan(&bitstring)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("list not found")
		}
		return nil, fmt.Errorf("get status list: %w", err)
	}

	return bitstring, nil
}

func (pc *postgresConnection) GetStatusListWithSigner(ctx context.Context, tenantId string, listId int) (*StatusListWithSigner, error) {
	var out StatusListWithSigner

	err := pc.conn.QueryRow(ctx, `
		SELECT
			tenant_id,
			list_id,
			type,
			version,
			bitstring,
			did,
			key_ref,
			namespace,
			COALESCE("group", ''),
			origin,
			COALESCE(purpose, ''),
			COALESCE(status_url, ''),
			max_expiration_date
		FROM status_lists
		WHERE tenant_id = $1
		  AND list_id = $2
		LIMIT 1
	`, tenantId, listId).Scan(
		&out.TenantID,
		&out.ListID,
		&out.Type,
		&out.Version,
		&out.Bitstring,
		&out.DID,
		&out.KeyRef,
		&out.Namespace,
		&out.Group,
		&out.Origin,
		&out.Purpose,
		&out.StatusURL,
		&out.MaxExpirationDate,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("list not found")
		}
		return nil, fmt.Errorf("get status list with signer: %w", err)
	}

	return &out, nil
}

func (pc *postgresConnection) RevokeCredentialInSpecifiedList(ctx context.Context, tenantId string, listId int, index int) error {
	tx, err := pc.conn.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.ReadCommitted,
		AccessMode: pgx.ReadWrite,
	})
	if err != nil {
		return fmt.Errorf("could not start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var bitstring []byte

	err = tx.QueryRow(ctx, `
		SELECT bitstring
		FROM status_lists
		WHERE tenant_id = $1
		  AND list_id = $2
		FOR UPDATE
	`, tenantId, listId).Scan(&bitstring)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("status list %d not found", listId)
		}
		return fmt.Errorf("select status list for revoke: %w", err)
	}

	list := entity.List{
		ListId: listId,
		List:   bitstring,
	}

	list.RevokeAtIndex(index)

	_, err = tx.Exec(ctx, `
		UPDATE status_lists
		SET bitstring = $1,
		    version = version + 1,
		    updated_at = now()
		WHERE tenant_id = $2
		  AND list_id = $3
	`, list.List, tenantId, listId)
	if err != nil {
		return fmt.Errorf("update revoked status list: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (pc *postgresConnection) CacheList(ctx context.Context, cacheId string, list []byte) error {
	_, err := pc.conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS status_list_cache (
			cache_id TEXT PRIMARY KEY,
			list BYTEA NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`)
	if err != nil {
		return fmt.Errorf("create status list cache table: %w", err)
	}

	_, err = pc.conn.Exec(ctx, `
		INSERT INTO status_list_cache (
			cache_id,
			list,
			updated_at
		)
		VALUES ($1, $2, now())
		ON CONFLICT (cache_id)
		DO UPDATE SET
			list = EXCLUDED.list,
			updated_at = now()
	`, cacheId, list)
	if err != nil {
		return fmt.Errorf("cache status list: %w", err)
	}

	return nil
}

func (pc *postgresConnection) CreateTableForTenantIdIfNotExists(ctx context.Context, tenantId string) error {
	return nil
}

func (pc *postgresConnection) Close() {
	pc.conn.Close()
}
