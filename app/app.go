package app

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/urfave/cli/v2"
	"go.uber.org/fx"
)

// Context is the minimal flag-access abstraction passed to modules during
// installation. It decouples Module implementations from a concrete CLI
// library so the framework can later swap urfave/cli for cobra (or be driven
// from tests) without touching modules.
type Context interface {
	Bool(name string) bool
	String(name string) string
	Int(name string) int
	Int64(name string) int64
	Float64(name string) float64
	Duration(name string) time.Duration
	StringSlice(name string) []string
	IsSet(name string) bool
}

// Module is a self-describing unit that can register its own CLI flags and
// produce an fx.Option graph at startup.
type Module interface {
	// Name is used to wrap the module in fx.Module(name, ...) for clearer
	// error messages and dependency graphs.
	Name() string
	// Configure is called once during app construction. Modules may register
	// flags or commands on the root cli.App here.
	Configure(app *cli.App)
	// Install is called when the app is run. It returns the fx.Options that
	// describe the module's providers / invokes / lifecycle hooks.
	Install(ctx Context) fx.Option
}

// Application is the top-level container that wires together a urfave/cli
// entry point and an uber/fx dependency-injection graph.
type Application struct {
	cliApp  *cli.App
	modules []Module
}

// NewApplication creates a new Application with the given name and usage
// description. The binary version is taken from the build-injected variables
// in version.go.
func NewApplication(name, description string) *Application {
	app := &cli.App{
		Name:    name,
		Usage:   description,
		Version: Version(),
	}

	cli.VersionPrinter = func(_ *cli.Context) {
		printBuildInfo()
	}

	return &Application{
		cliApp: app,
	}
}

// Install registers one or more Modules with the application. The order of
// installation is preserved when building the fx graph.
func (app *Application) Install(modules ...Module) {
	app.modules = append(app.modules, modules...)
}

// Run parses os.Args and starts the application.
func (app *Application) Run() error {
	app.configure()
	return app.cliApp.Run(os.Args)
}

func (app *Application) configure() {
	app.cliApp.Action = app.action
	for _, module := range app.modules {
		module.Configure(app.cliApp)
	}
}

func (app *Application) action(cCtx *cli.Context) error {
	opts := make([]fx.Option, 0, len(app.modules)+1)
	for _, m := range app.modules {
		opts = append(opts, fx.Module(m.Name(), m.Install(cCtx)))
	}
	opts = append(opts, fx.StartTimeout(180*time.Second))

	fxApp := fx.New(opts...)
	if err := fxApp.Err(); err != nil {
		return fmt.Errorf("build fx app: %w", err)
	}
	fxApp.Run()
	return nil
}

// RunService is a helper for modules that need to run a blocking workload
// (e.g. an HTTP server, a consumer loop). It wires the worker into fx's
// lifecycle so that:
//   - OnStart launches the worker in its own goroutine,
//   - OnStop signals it via context cancellation and waits for it to exit.
//
// The worker function MUST return when the supplied context is cancelled.
func RunService(lc fx.Lifecycle, run func(ctx context.Context) error) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			go func() {
				defer close(done)
				if err := run(ctx); err != nil && ctx.Err() == nil {
					// Worker exited with an error before shutdown was
					// requested. Log to stderr; callers wanting richer
					// handling should use fx.Shutdowner directly.
					fmt.Fprintf(os.Stderr, "webx: service exited: %v\n", err)
				}
			}()
			return nil
		},
		OnStop: func(stopCtx context.Context) error {
			cancel()
			select {
			case <-done:
				return nil
			case <-stopCtx.Done():
				return stopCtx.Err()
			}
		},
	})
}
