package logs

// Default values for the logs module. They are applied when the corresponding
// CLI flag / env var is not set.
const (
	ModuleName = "logs"

	DefaultLevel  = "info" // debug | info | warn | error | panic
	DefaultFormat = "text" // text | json
)
