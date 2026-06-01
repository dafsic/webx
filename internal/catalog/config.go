package main

// Module / flag identifiers and defaults for the catalog service.
const (
	moduleName = "catalog"

	defaultGRPCAddr = ":50052"
)

// Config is the resolved configuration for the catalog service.
type Config struct {
	// GRPCAddr is the listen address of the gRPC server.
	GRPCAddr string
}
