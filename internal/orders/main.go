package main

import (
	"log"

	"github.com/dafsic/webx/app"
	"github.com/dafsic/webx/modules/logs"
	"github.com/dafsic/webx/modules/postgres"
)

func main() {
	a := app.NewApplication("orders", "Webx bar ordering service")
	a.Install(
		logs.New(),
		postgres.New(),
		New(),
	)
	if err := a.Run(); err != nil {
		log.Fatal(err)
	}
}
