package main

// Module / flag identifiers and defaults for the HTTP gateway.
const (
	moduleName = "gateway"

	defaultHTTPAddr     = ":8080"
	defaultPeopleAddr   = "127.0.0.1:50051"
	defaultOpenAPISpec  = "./proto_go/webx.swagger.json"
	defaultCORSOrigins  = "*"
)

// Config is the resolved configuration for the gateway.
type Config struct {
	// HTTPAddr is the listen address of the public HTTP server.
	HTTPAddr string
	// JWTSecret is the HS256 secret shared with the people service, used by the
	// centralized auth middleware to validate session tokens.
	JWTSecret string
	// PeopleAddr is the gRPC endpoint of the people microservice.
	PeopleAddr string
	// OpenAPISpec is the path to the generated swagger JSON served under /docs.
	// Empty or missing file disables the docs endpoints.
	OpenAPISpec string
	// CORSOrigins is the list of allowed origins. A single "*" allows any
	// origin (credentials are then disabled per the CORS spec).
	CORSOrigins []string
}
