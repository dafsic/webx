// Command gateway is the HTTP gateway of the webx platform. It receives
// REST traffic, verifies JWTs and forwards calls to internal gRPC services
// via grpc-gateway.
package main

import (
	"fmt"
	"os"

	"github.com/dafsic/webx/app"
	gw "github.com/dafsic/webx/internal/gateway"
	"github.com/dafsic/webx/modules/ginserver"
	"github.com/dafsic/webx/modules/grpcclient"
	"github.com/dafsic/webx/modules/logs"
	"github.com/dafsic/webx/modules/otel"
)

func main() {
	a := app.NewApplication("gateway",
		"webx http gateway: gin + grpc-gateway + swagger")

	a.Install(
		logs.New(),
		otel.New(),
		grpcclient.New(),
		ginserver.New(),
		gw.New(),
	)

	if err := a.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
