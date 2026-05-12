# app

`app` 包是 webx 微服务框架的内核，负责把 [`urfave/cli`](https://github.com/urfave/cli) 命令行入口与 [`uber-go/fx`](https://github.com/uber-go/fx) 依赖注入容器编排在一起。它对外只暴露三个核心概念：

- `Application` —— 进程级容器，负责命令行解析与 fx 图的构建。
- `Module` —— 自描述的功能单元，可注册自己的 CLI flag，并产出一组 `fx.Option`。
- `RunService` —— 把"阻塞型工作负载"挂入 fx 生命周期的辅助函数。

## 快速开始

```go
package main

import (
    "log"

    "github.com/dafsic/webx/app"
)

func main() {
    a := app.NewApplication("myservice", "My awesome service")
    a.Install(
        // logger.New(),
        // config.New(),
        // httpserver.New(),
    )
    if err := a.Run(); err != nil {
        log.Fatal(err)
    }
}
```

构建时通过 `-ldflags` 注入版本信息（变量定义见 [version.go](version.go)）：

```sh
go build -ldflags "\
  -X github.com/dafsic/webx/app.version=v1.0.0 \
  -X github.com/dafsic/webx/app.goVersion=$(go version | awk '{print $3}') \
  -X github.com/dafsic/webx/app.buildTime=$(date -u +%FT%TZ) \
  -X github.com/dafsic/webx/app.commitHash=$(git rev-parse HEAD) \
  -X github.com/dafsic/webx/app.gitBranch=$(git rev-parse --abbrev-ref HEAD) \
  -X github.com/dafsic/webx/app.gitTreeState=clean"
```

运行 `./myservice --version` 会打印完整的构建信息。

## 核心 API

### `Application`

| 方法 | 说明 |
| --- | --- |
| `NewApplication(name, usage string) *Application` | 创建容器，自动接管 `--version`。 |
| `Install(modules ...Module)` | 注册一个或多个模块；保留注册顺序。 |
| `Run() error` | 解析 `os.Args` 并启动；fx 构图失败会作为错误返回。 |

### `Module`

```go
type Module interface {
    Name() string                       // 用作 fx.Module(name, ...) 的标签
    Configure(app *cli.App)             // 注册 flag / 子命令
    Install(ctx Context) fx.Option      // 返回该模块的 fx 选项
}
```

注意 `Install` 的参数是 `app.Context` 接口，而不是 `*cli.Context`，目的是让模块与具体的 CLI 库解耦。`*cli.Context` 已经实现了该接口，无需手动转换。

### `Context`

```go
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
```

只暴露读取 flag 所需的最小子集；要扩展时请保持向后兼容。

### `RunService`

把"长期运行 + 需要优雅退出"的工作负载交给 fx 生命周期：

```go
func (m *HTTPModule) Install(ctx app.Context) fx.Option {
    addr := ctx.String("http-addr")
    return fx.Options(
        fx.Provide(func() *Server { return NewServer(addr) }),
        fx.Invoke(func(lc fx.Lifecycle, s *Server) {
            app.RunService(lc, s.Serve) // s.Serve(ctx context.Context) error
        }),
    )
}
```

语义：

- `OnStart` 在独立 goroutine 中调用 `run(ctx)`。
- `OnStop` 取消 `ctx` 并等待 worker 退出；若超过 fx 的 stop timeout，则返回 `context.DeadlineExceeded`。
- worker **必须**在 `ctx` 被取消后及时返回，否则会拖慢整个进程退出。
- 若需要在异常退出时触发整进程关闭，请在 worker 内注入 `fx.Shutdowner` 显式调用。

## 编写一个模块

```go
package httpserver

import (
    "context"
    "net/http"

    "github.com/dafsic/webx/app"
    "github.com/urfave/cli/v2"
    "go.uber.org/fx"
)

type Module struct{}

func New() *Module { return &Module{} }

func (m *Module) Name() string { return "httpserver" }

func (m *Module) Configure(a *cli.App) {
    a.Flags = append(a.Flags, &cli.StringFlag{
        Name:    "http-addr",
        EnvVars: []string{"HTTP_ADDR"},
        Value:   ":8080",
    })
}

func (m *Module) Install(ctx app.Context) fx.Option {
    addr := ctx.String("http-addr")
    return fx.Options(
        fx.Provide(func() *http.Server {
            return &http.Server{Addr: addr}
        }),
        fx.Invoke(func(lc fx.Lifecycle, s *http.Server) {
            app.RunService(lc, func(c context.Context) error {
                go func() { <-c.Done(); _ = s.Shutdown(context.Background()) }()
                if err := s.ListenAndServe(); err != http.ErrServerClosed {
                    return err
                }
                return nil
            })
        }),
    )
}
```

## 设计准则

1. **每个模块自管 flag**：框架不再硬编码业务相关 flag（例如 `--migrate`），需要哪些参数由模块自行 `Configure`。
2. **模块即 fx 子图**：`Install` 返回的 `fx.Option` 会被自动包进 `fx.Module(name, ...)`，错误信息和 dotgraph 都会带上模块名。
3. **不要在 `Install` 里做 IO**：`Install` 只声明依赖图，真正的连接、监听等动作放在 `fx.Lifecycle.OnStart` 中。
4. **不要阻塞主线程**：所有 long-running 工作必须经由 `RunService` 或自定义的 `fx.Hook` 启动 goroutine。
5. **错误向上回传**：`fx.New` 构图失败会从 `Application.Run()` 返回 `error`，调用方有机会决定退出码或重试。

## 版本信息

`app.Version()` 返回纯版本字符串；`app.GetBuildInfo()` 返回完整结构体，便于在 `/healthz`、日志等场景中复用。
