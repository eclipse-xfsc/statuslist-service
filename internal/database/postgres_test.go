package database

//import (
//	"context"
//	"database/sql"
//	"fmt"
//	_ "github.com/lib/pq"
//	"github.com/ory/dockertest/v3"
//	"github.com/ory/dockertest/v3/docker"
//	"github.com/stretchr/testify/require"
//	"github.com/eclipse-xfsc/statuslist-service/entity"
//	"log"
//	"os"
//	"testing"
//	"time"
//)
//
//currently comment out cause of no dind in gitlab runner
//
//var db *sql.DB
//var databaseUrl string
//
//const dbUser = "root"
//const dbPassword = "ineedcoffee"
//const dbName = "status"
//const tenantId = "5"
//const listSizeInBytes = 10
//
//func TestMain(m *testing.M) {
//	// uses a sensible default on windows (tcp/http) and linux/osx (socket)
//	pool, err := dockertest.NewPool("")
//	if err != nil {
//		log.Fatalf("Could not construct pool: %s", err)
//	}
//
//	if err = pool.Client.Ping(); err != nil {
//		log.Fatalf("Could not connect to Docker: %s", err)
//	}
//
//	// pulls an image, creates a container based on it and runs it
//	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
//		Repository: "postgres",
//		Tag:        "11",
//		Env: []string{
//			"POSTGRES_PASSWORD=" + dbPassword,
//			"POSTGRES_USER=" + dbUser,
//			"POSTGRES_DB=" + dbName,
//			"listen_addresses = '*'",
//		},
//	}, func(config *docker.HostConfig) {
//		// set AutoRemove to true so that stopped container goes away by itself
//		config.AutoRemove = true
//		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
//	})
//	if err != nil {
//		log.Fatalf("Could not start resource: %s", err)
//	}
//
//	hostAndPort := getHostPort(resource, "5432/tcp")
//	databaseUrl = fmt.Sprintf("postgres://%s:%s@%s/%s", dbUser, dbPassword, hostAndPort, dbName)
//	databaseUrlSsl := databaseUrl + "?sslmode=disable"
//
//	log.Println("Connecting to database on url: ", databaseUrlSsl)
//
//	resource.Expire(120) // Tell docker to hard kill the container in 120 seconds
//
//	// exponential backoff-retry, because the application in the container might not be ready to accept connections yet
//	pool.MaxWait = 120 * time.Second
//	if err = pool.Retry(func() error {
//		db, err = sql.Open("postgres", databaseUrlSsl)
//		if err != nil {
//			return err
//		}
//		return db.Ping()
//	}); err != nil {
//		log.Fatalf("Could not connect to database: %s", err)
//	}
//
//	//Run tests
//	code := m.Run()
//
//	// You can't defer this because os.Exit doesn't care for defer
//	if err := pool.Purge(resource); err != nil {
//		log.Fatalf("Could not purge resource: %s", err)
//	}
//
//	os.Exit(code)
//}
//
//func getHostPort(resource *dockertest.Resource, id string) string {
//	dInD := os.Getenv("DIND_SERVICE_NAME")
//	if dInD == "" {
//		return resource.GetHostPort(id)
//	}
//	return dInD + ":" + resource.GetPort(id)
//}
//
//func prepareDatabase() error {
//	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{
//		Isolation: 0,
//		ReadOnly:  false,
//	})
//	if err != nil {
//		return fmt.Errorf("could not create transaction: %w", err)
//	}
//	defer tx.Rollback()
//
//	createTableQuery := fmt.Sprintf("CREATE TABLE %s (listID SERIAL PRIMARY KEY, list BYTEA, free INT)", TablePrefix+tenantId)
//	_, err = tx.Exec(createTableQuery)
//	if err != nil {
//		return fmt.Errorf("could not create new table for tenantID: %w", err)
//	}
//
//	newList := entity.NewList(listSizeInBytes)
//	insertQuery := fmt.Sprintf("INSERT INTO %s (list, free) VALUES ($1, $2)", TablePrefix+tenantId)
//	_, err = tx.Exec(insertQuery, newList.List, newList.Free)
//	if err != nil {
//		return fmt.Errorf("error inserting new list into the database: %w", err)
//	}
//
//	err = tx.Commit()
//	if err != nil {
//		return fmt.Errorf("error commiting transaction: %w", err)
//	}
//	return nil
//}
//
//func cleanupDatabase() error {
//	dropTableQuery := fmt.Sprintf("DROP TABLE %s", TablePrefix+tenantId)
//	_, err := db.Exec(dropTableQuery)
//	if err != nil {
//		return fmt.Errorf("could not drop table: %w", err)
//	}
//
//	return nil
//}
//
//func TestTableCheckAndCreation(t *testing.T) {
//	database, err := New(databaseUrl, listSizeInBytes)
//	if err != nil {
//		t.Errorf("unexpected error occured: %v", err)
//	}
//	defer func() {
//		err := cleanupDatabase()
//		if err != nil {
//			t.Errorf("unexpected error occured: %v", err)
//		}
//	}()
//
//	err = database.DbConnection.CreateTableForTenantIdIfNotExists(context.Background(), tenantId)
//	if err != nil {
//		t.Errorf("unexpected error occured: %v", err)
//	}
//
//	var n int64
//	err = db.QueryRow("SELECT 1 FROM information_schema.tables WHERE table_name = $1", TablePrefix+tenantId).Scan(&n)
//	require.Nil(t, err)
//
//	var listId int
//	err = db.QueryRow(fmt.Sprintf("SELECT listID FROM %s WHERE free = %d", TablePrefix+tenantId, listSizeInBytes*8)).Scan(&listId)
//	require.Nil(t, err)
//}
//
//func TestAllocateIndex(t *testing.T) {
//	want := entity.NewStatusData(0, 1)
//
//	database, err := New(databaseUrl, listSizeInBytes)
//	if err != nil {
//		t.Errorf("unexpected error occured: %v", err)
//	}
//
//	err = prepareDatabase()
//	if err != nil {
//		t.Errorf("unexpected error occured: %v", err)
//	}
//	defer func() {
//		err := cleanupDatabase()
//		if err != nil {
//			t.Errorf("unexpected error occured: %v", err)
//		}
//	}()
//
//	got, err := database.DbConnection.AllocateIndexInCurrentList(context.Background(), tenantId)
//	if err != nil {
//		t.Errorf("unexpected error occured: %v", err)
//	}
//
//	require.Equal(t, want, got)
//}
//
//func TestRevokeCredential(t *testing.T) {
//	index := 0
//
//	newList := entity.NewList(listSizeInBytes)
//	newList.RevokeAtIndex(index)
//	want := newList.List
//
//	database, err := New(databaseUrl, listSizeInBytes)
//	if err != nil {
//		t.Errorf("unexpected error occured: %v", err)
//	}
//
//	err = prepareDatabase()
//	if err != nil {
//		t.Errorf("unexpected error occured: %v", err)
//	}
//	defer func() {
//		err = cleanupDatabase()
//		if err != nil {
//			t.Errorf("unexpected error occured: %v", err)
//		}
//	}()
//
//	err = database.DbConnection.RevokeCredentialInSpecifiedList(context.Background(), tenantId, 1, index)
//	if err != nil {
//		t.Errorf("unexpected error occured: %v", err)
//	}
//
//	var got []byte
//	err = db.QueryRow(fmt.Sprintf("SELECT list FROM %s WHERE listID = %d", TablePrefix+tenantId, 1)).Scan(&got)
//	require.Equal(t, want, got)
//}
