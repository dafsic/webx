package logs

// Default values for the logs module. They are applied when the corresponding
// CLI flag / env var is not set.
const (
	ModuleName = "logs"

	FlagLevel  = "log-level"
	FlagFormat = "log-format"

	EnvLevel  = "LOG_LEVEL"
	EnvFormat = "LOG_FORMAT"

	DefaultLevel  = "info" // debug | info | warn | error | panic
	DefaultFormat = "text" // text | json

	// OptionsGroup is the fx group tag callers can use to inject extra
	// Options into the logger configuration. Example:
	//
	//	fx.Supply(fx.Annotate(
	//	    logs.WithLevel("debug"),
	//	    fx.ResultTags(`group:"`+logs.OptionsGroup+`"`),
	//	))
	OptionsGroup = "logs_options"
)
