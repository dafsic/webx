package app

import "fmt"

// These variables are populated at build time via -ldflags, e.g.:
//
//	go build -ldflags "-X github.com/dafsic/webx/app.version=v1.0.0 \
//	                   -X github.com/dafsic/webx/app.goVersion=$(go version | awk '{print $3}') \
//	                   -X github.com/dafsic/webx/app.buildTime=$(date -u +%FT%TZ) \
//	                   -X github.com/dafsic/webx/app.commitHash=$(git rev-parse HEAD) \
//	                   -X github.com/dafsic/webx/app.gitBranch=$(git rev-parse --abbrev-ref HEAD) \
//	                   -X github.com/dafsic/webx/app.gitTreeState=clean"
var (
	version      string
	goVersion    string
	buildTime    string
	commitHash   string
	gitBranch    string
	gitTreeState string
)

// BuildInfo describes the build metadata of the current binary.
type BuildInfo struct {
	Version      string
	GoVersion    string
	BuildTime    string
	CommitHash   string
	GitBranch    string
	GitTreeState string
}

// GetBuildInfo returns the build metadata baked into the binary at link time.
func GetBuildInfo() BuildInfo {
	return BuildInfo{
		Version:      version,
		GoVersion:    goVersion,
		BuildTime:    buildTime,
		CommitHash:   commitHash,
		GitBranch:    gitBranch,
		GitTreeState: gitTreeState,
	}
}

// Version returns the binary version string.
func Version() string {
	return version
}

func printBuildInfo() {
	info := GetBuildInfo()
	fmt.Printf("VERSION:         %s\n", info.Version)
	fmt.Printf("GO_VERSION:      %s\n", info.GoVersion)
	fmt.Printf("GIT_BRANCH:      %s\n", info.GitBranch)
	fmt.Printf("COMMIT_HASH:     %s\n", info.CommitHash)
	fmt.Printf("GIT_TREE_STATE:  %s\n", info.GitTreeState)
	fmt.Printf("BUILD_TIME:      %s\n", info.BuildTime)
}
