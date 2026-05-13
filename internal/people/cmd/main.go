// Command people is the entry point of the people microservice.
package main

import (
	"fmt"
	"os"

	"github.com/dafsic/webx/app"
	peopleapi "github.com/dafsic/webx/internal/people/api"
	peoplegrpc "github.com/dafsic/webx/internal/people/server"
	"github.com/dafsic/webx/modules/grpcserver"
	"github.com/dafsic/webx/modules/logs"
	"github.com/dafsic/webx/modules/otel"
	"github.com/dafsic/webx/modules/postgres"
	"github.com/dafsic/webx/modules/redis"
)

func main() {
	a := app.NewApplication("people",
		"people microservice: gRPC login backed by postgres + redis (JWT)")

	a.Install(
		logs.New(),
		otel.New(),
		postgres.New(),
		redis.New(),
		grpcserver.New(),
		peopleapi.New(),
		peoplegrpc.New(),
	)

	if err := a.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
