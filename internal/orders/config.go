package main

// Module / flag identifiers and defaults for the orders service.
const (
	moduleName = "orders"

	defaultGRPCAddr    = ":50053"
	defaultCatalogAddr = "127.0.0.1:50052"
)

// Config is the resolved configuration for the orders service.
type Config struct {
	// GRPCAddr is the listen address of the gRPC server.
	GRPCAddr string
	// CatalogAddr is the gRPC endpoint of the catalog service, used to resolve
	// menu item names and prices when an order is created.
	CatalogAddr string
}
