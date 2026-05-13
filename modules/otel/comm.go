package otel

// Module identity & CLI flag identifiers.
const (
	ModuleName = "otel"

	FlagEndpoint     = "otel-endpoint"
	FlagServiceName  = "otel-service-name"
	FlagSampleRatio  = "otel-sample-ratio"
	FlagInsecure     = "otel-insecure"
	FlagDisabled     = "otel-disabled"

	EnvEndpoint     = "OTEL_ENDPOINT"
	EnvServiceName  = "OTEL_SERVICE_NAME"
	EnvSampleRatio  = "OTEL_SAMPLE_RATIO"
	EnvInsecure     = "OTEL_INSECURE"
	EnvDisabled     = "OTEL_SDK_DISABLED"

	DefaultEndpoint    = "localhost:4317"
	DefaultSampleRatio = 1.0

	// OptionsGroup is the fx group name external callers use to contribute
	// extra Options.
	OptionsGroup = "otel_options"
)
