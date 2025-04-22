package database

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"time"

	ctxPkg "github.com/eclipse-xfsc/microservice-core-go/pkg/ctx"
	pgPkg "github.com/eclipse-xfsc/microservice-core-go/pkg/db/postgres"
	errPkg "github.com/eclipse-xfsc/microservice-core-go/pkg/err"
	"github.com/eclipse-xfsc/statuslist-service/internal/entity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TODO: queries as constants?

type postgresConnection struct {
	conn            *pgxpool.Pool
	listSizeInBytes int
}

func (pc *postgresConnection) Ping() bool {
	return pc.conn.Ping(context.Background()) == nil
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

	return &postgresConnection{
		conn:            conn,
		listSizeInBytes: listSizeInBytes,
	}, nil
}

func (pc *postgresConnection) GetStatusList(ctx context.Context, tenantId string, listId int) ([]byte, error) {
	tx, err := pc.conn.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:       pgx.ReadCommitted,
		AccessMode:     pgx.ReadWrite,
		DeferrableMode: pgx.NotDeferrable,
	})

	if err != nil {
		return nil, fmt.Errorf("could not start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err != nil {
		return nil, fmt.Errorf("error creating transaction: %w", err)
	}

	tableName, err := createTableName(tenantId)
	if err != nil {
		return nil, err
	}

	selectQuery := fmt.Sprintf("SELECT listID,list, free FROM %s WHERE listID=%s LIMIT 1", tableName, strconv.Itoa(listId))

	rows, err := tx.Query(ctx, selectQuery)
	if err != nil {
		return nil, fmt.Errorf("error while select current list from the database: %w", err)
	}

	databaseRows, err := pgx.CollectRows(rows, pgx.RowToStructByName[entity.List])
	if err != nil {
		return nil, fmt.Errorf("error while collecting current list from rows: %w", err)
	}

	if len(databaseRows) == 0 {
		return nil, errors.New("list not found")
	}

	currentList := databaseRows[0]

	return currentList.List, nil
}

func (pc *postgresConnection) AllocateIndexInCurrentList(ctx context.Context, tenantId string) (*entity.StatusData, error) {
	tx, err := pc.conn.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:       pgx.ReadCommitted,
		AccessMode:     pgx.ReadWrite,
		DeferrableMode: pgx.NotDeferrable,
	})
	if err != nil {
		return nil, fmt.Errorf("could not start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err != nil {
		return nil, fmt.Errorf("error creating transaction: %w", err)
	}

	tableName, err := createTableName(tenantId)
	if err != nil {
		return nil, err
	}

	selectQuery := fmt.Sprintf("SELECT listID, list, free FROM %s WHERE free > 0 FOR UPDATE LIMIT 1", tableName)
	rows, err := tx.Query(ctx, selectQuery)
	if err != nil {
		return nil, fmt.Errorf("error while select current list from the database: %w", err)
	}
	// not optimized for performance cause of reflection
	databaseRows, err := pgx.CollectRows(rows, pgx.RowToStructByName[entity.List])
	if err != nil {
		return nil, fmt.Errorf("error while collecting current list from rows: %w", err)
	}

	if len(databaseRows) == 0 {
		// no current list -> create new one and allocate index
		newList := entity.NewList(pc.listSizeInBytes)

		index, err := newList.AllocateNextFreeIndex()
		if err != nil {
			return nil, fmt.Errorf("error allocating next free index from new list: %w", err)
		}

		insertQuery := fmt.Sprintf("INSERT INTO %s (list, free) VALUES ($1, $2) RETURNING listID", tableName)
		var listId int
		if err = tx.
			QueryRow(ctx, insertQuery, newList.List, newList.Free).
			Scan(&listId); err != nil {
			return nil, fmt.Errorf("error inserting new list into the database: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("error commiting transaction: %w", err)
		}

		return entity.NewStatusData(index, listId), nil
	}

	// allocate index in current list
	currentList := databaseRows[0]

	index, err := currentList.AllocateNextFreeIndex()
	if err != nil {
		return nil, fmt.Errorf("error allocating next free index from current list: %w", err)
	}

	updateQuery := fmt.Sprintf("UPDATE %s%s SET list = $1, free = $2 WHERE listID = $3", TablePrefix, tenantId)
	if _, err := tx.Exec(ctx, updateQuery, currentList.List, currentList.Free, currentList.ListId); err != nil {
		return nil, fmt.Errorf("error updating list in the database: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("error commiting transaction: %w", err)
	}

	return entity.NewStatusData(index, currentList.ListId), nil
}

func (pc *postgresConnection) RevokeCredentialInSpecifiedList(ctx context.Context, tenantId string, listId int, index int) error {
	tx, err := pc.conn.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:       pgx.ReadCommitted,
		AccessMode:     pgx.ReadWrite,
		DeferrableMode: pgx.NotDeferrable,
	})
	if err != nil {
		return fmt.Errorf("could not start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	tableName, err := createTableName(tenantId)
	if err != nil {
		return err
	}

	fmt.Println(tableName)
	selectQuery := fmt.Sprintf("SELECT listID, list, free FROM %s WHERE listID = $1 FOR UPDATE LIMIT 1", tableName)
	rows, err := tx.Query(ctx, selectQuery, listId)
	if err != nil {
		return fmt.Errorf("error while select specified list from the database: %w", err)
	}
	databaseRows, err := pgx.CollectRows(rows, pgx.RowToStructByName[entity.List])
	if err != nil {
		return fmt.Errorf("error while getting specified list from rows: %w", err)
	}

	if len(databaseRows) == 0 {
		return fmt.Errorf("listId %d does not exist in database", listId)
	}

	specifiedList := databaseRows[0]

	specifiedList.RevokeAtIndex(index)

	updateQuery := fmt.Sprintf("UPDATE %s%s SET list = $1 WHERE listID = $2", TablePrefix, tenantId)
	_, err = tx.Exec(ctx, updateQuery, specifiedList.List, specifiedList.ListId)
	if err != nil {
		return fmt.Errorf("error updating list in the database: %w", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("error commiting transaction: %w", err)
	}

	return nil
}

func (pc *postgresConnection) CacheList(ctx context.Context, cacheId string, list []byte) error {
	tx, err := pc.conn.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:       pgx.ReadCommitted,
		AccessMode:     pgx.ReadWrite,
		DeferrableMode: pgx.NotDeferrable,
	})

	if err != nil {
		return fmt.Errorf("could not start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var n int64
	exists := true

	_, err = tx.Exec(ctx, "LOCK TABLE information_schema.tables IN EXCLUSIVE MODE")
	if err != nil {
		return fmt.Errorf("could not lock table: %w", err)
	}

	tableName, err := createTableName(cacheId)
	if err != nil {
		return err
	}

	const tableExistQuery = "SELECT 1 FROM information_schema.tables WHERE table_name = $1"
	if err = tx.QueryRow(ctx, tableExistQuery, tableName).Scan(&n); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			exists = false
		} else {
			return fmt.Errorf("error query for table name: %w", err)
		}
	}

	if !exists {
		createTableQuery := fmt.Sprintf("CREATE TABLE %s (listID SERIAL PRIMARY KEY, list BYTEA, lastupdate timestamp)", tableName)
		_, err = tx.Exec(ctx, createTableQuery)
		if err != nil {
			return fmt.Errorf("could not create new table for tenantID: %w", err)
		}

		insertQuery := fmt.Sprintf("INSERT INTO %s (list, lastupdate) VALUES ($1, $2)", tableName)
		_, err = tx.Exec(ctx, insertQuery, list, time.Now())
		if err != nil {
			return fmt.Errorf("error inserting new list into the database: %w", err)
		}
	} else {
		updateQuery := fmt.Sprintf("UPDATE %s SET list=$1, lastupdate=$2", tableName)
		_, err = tx.Exec(ctx, updateQuery, list, time.Now())
		if err != nil {
			return fmt.Errorf("error inserting new list into the database: %w", err)
		}
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("error commiting transaction: %w", err)
	}

	return nil
}

func (pc *postgresConnection) CreateTableForTenantIdIfNotExists(ctx context.Context, tenantId string) error {
	tx, err := pc.conn.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:       pgx.ReadCommitted,
		AccessMode:     pgx.ReadWrite,
		DeferrableMode: pgx.NotDeferrable,
	})
	if err != nil {
		return fmt.Errorf("could not start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var n int64
	exists := true

	_, err = tx.Exec(ctx, "LOCK TABLE information_schema.tables IN EXCLUSIVE MODE")
	if err != nil {
		return fmt.Errorf("could not lock table: %w", err)
	}

	tableName, err := createTableName(tenantId)
	if err != nil {
		return err
	}

	const tableExistQuery = "SELECT 1 FROM information_schema.tables WHERE table_name = $1"
	if err = tx.QueryRow(ctx, tableExistQuery, tableName).Scan(&n); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			exists = false
		} else {
			return fmt.Errorf("error query for table name: %w", err)
		}
	}

	if !exists {
		createTableQuery := fmt.Sprintf("CREATE TABLE %s (listID SERIAL PRIMARY KEY, list BYTEA, free INT)", tableName)
		_, err = tx.Exec(ctx, createTableQuery)
		if err != nil {
			return fmt.Errorf("could not create new table for tenantID: %w", err)
		}

		newList := entity.NewList(pc.listSizeInBytes)

		insertQuery := fmt.Sprintf("INSERT INTO %s (list, free) VALUES ($1, $2)", tableName)
		_, err = tx.Exec(ctx, insertQuery, newList.List, newList.Free)
		if err != nil {
			return fmt.Errorf("error inserting new list into the database: %w", err)
		}
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("error commiting transaction: %w", err)
	}

	return nil
}

func (pc *postgresConnection) Close() {
	pc.conn.Close()
}

func createTableName(tenantId string) (string, error) {
	tableName := TablePrefix + tenantId
	isValid, err := regexp.Match("^[a-zA-Z0-9_]+$", []byte(tableName))
	if err != nil {
		return "", fmt.Errorf("error while checking tableName: %w", err)
	}

	if !isValid {
		return "", fmt.Errorf("tableName '%s' is not valid", tableName)
	}

	return tableName, nil
}
