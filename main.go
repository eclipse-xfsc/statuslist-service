package main

import (
	"context"

	ctxPkg "github.com/eclipse-xfsc/microservice-core-go/pkg/ctx"
	logPkg "github.com/eclipse-xfsc/microservice-core-go/pkg/logr"
	"github.com/eclipse-xfsc/statuslist-service/internal/api"
	"github.com/eclipse-xfsc/statuslist-service/internal/config"
	"github.com/eclipse-xfsc/statuslist-service/internal/database"
	log "github.com/sirupsen/logrus"
)

func main() {
	ctx := context.Background()

	if err := config.Load(); err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	currentConf := &config.CurrentStatusListConfig

	logger, err := logPkg.New(currentConf.LogLevel, currentConf.IsDev, nil)
	if err != nil {
		log.Fatalf("failed to init logger: %v", err)
	}

	ctx = ctxPkg.WithLogger(ctx, *logger)
	config.SetLogger(*logger)
	dbConf := &currentConf.Database

	db, err := database.New(ctx, *dbConf, currentConf.ListSizeInBytes)

	if err != nil {
		log.Fatalf("database cant be established: %v", err)
	}

	api.Listen(db, currentConf)
}
