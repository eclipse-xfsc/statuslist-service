package api

import (
	"sync"

	"github.com/eclipse-xfsc/statuslist-service/internal/config"
	"github.com/eclipse-xfsc/statuslist-service/internal/database"
)

var db *database.Database

func Listen(database *database.Database, conf *config.StatusListConfiguration) {
	var wg sync.WaitGroup

	db = database

	wg.Add(2)
	go startMessaging(conf, &wg)

	go startRest(conf, &wg, db)

	wg.Wait()
}
