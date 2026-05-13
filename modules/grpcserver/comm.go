package grpcserver

const (
	ModuleName = "grpcserver"

	FlagAddr        = "grpc-addr"
	FlagReflection  = "grpc-reflection"
	FlagHealth      = "grpc-health"
	FlagStopTimeout = "grpc-stop-timeout"

	EnvAddr        = "GRPC_ADDR"
	EnvReflection  = "GRPC_REFLECTION"
	EnvHealth      = "GRPC_HEALTH"
	EnvStopTimeout = "GRPC_STOP_TIMEOUT"

	DefaultAddr        = ":50051"
	DefaultReflection  = true
	DefaultHealth      = true

	OptionsGroup = "grpcserver_options"
)
