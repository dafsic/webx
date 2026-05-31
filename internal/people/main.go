package main

import (
	"log"

	"github.com/dafsic/webx/app"
	"github.com/dafsic/webx/modules/logs"
	"github.com/dafsic/webx/modules/postgres"
	"github.com/dafsic/webx/modules/redis"
)

func main() {
	a := app.NewApplication("people", "Webx EVM-wallet authentication & RBAC service")
	a.Install(
		logs.New(),
		postgres.New(),
		redis.New(),
		New(),
	)
	if err := a.Run(); err != nil {
		log.Fatal(err)
	}
}
