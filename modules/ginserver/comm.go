package ginserver

const (
	ModuleName = "ginserver"

	FlagAddr        = "http-addr"
	FlagMode        = "gin-mode"
	FlagStopTimeout = "http-stop-timeout"
	FlagTrustedProxies = "http-trusted-proxies"

	EnvAddr        = "HTTP_ADDR"
	EnvMode        = "GIN_MODE"
	EnvStopTimeout = "HTTP_STOP_TIMEOUT"
	EnvTrustedProxies = "HTTP_TRUSTED_PROXIES"

	DefaultAddr = ":8080"
	DefaultMode = "release"

	OptionsGroup = "ginserver_options"
)
