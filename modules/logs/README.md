# logs

基于 [`log/slog`](https://pkg.go.dev/log/slog) 的日志模块，是 webx 框架的标准日志组件。

## 特性

- 5 个日志级别：`debug | info | warn | error | panic`（`panic` 通过自定义 `LevelPanic` 实现，slog 原生没有）。
- 两种输出格式：`text` / `json`。
- 通过 `--log-level` / `--log-format` 或环境变量 `LOG_LEVEL` / `LOG_FORMAT` 配置。
- 运行时可变级别：模块提供 `*slog.LevelVar`，其它模块（如 HTTP 管理端点）可直接 `Set` 调整。
- 函数式选项 + `fx` group：外部模块可注入额外配置而无需 fork。
- 注册时自动 `slog.SetDefault(logger)`，包级 `slog.Info(...)` 等函数共享同一配置。

## CLI

| Flag | Env | 默认 | 取值 |
| --- | --- | --- | --- |
| `--log-level` | `LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` / `panic` |
| `--log-format` | `LOG_FORMAT` | `text` | `text` / `json` |

## 快速开始

```go
package main

import (
    "log"

    "github.com/dafsic/webx/app"
    "github.com/dafsic/webx/modules/logs"
)

func main() {
    a := app.NewApplication("myservice", "demo")
    a.Install(logs.New())
    if err := a.Run(); err != nil {
        log.Fatal(err)
    }
}
```

```sh
./myservice --log-level=debug --log-format=json
LOG_LEVEL=warn ./myservice
```

## 在其它模块中使用

`logs.Module` 向 fx 图提供以下类型，按需注入即可：

| 类型 | 用途 |
| --- | --- |
| `*slog.Logger` | 主日志器 |
| `*slog.LevelVar` | 运行时动态调级 |
| `*logs.Config` | 已解析的配置（少用） |

```go
fx.Invoke(func(l *slog.Logger) {
    l.Info("server started", "addr", ":8080")
})
```

动态调级示例：

```go
fx.Invoke(func(lv *slog.LevelVar) {
    lv.Set(slog.LevelDebug) // 进入排查模式
})
```

## 扩展配置

模块用 `fx` group `logs.OptionsGroup`（值 `"logs_options"`）收集所有 `logs.Option`。CLI flag 派生的 options 已经在组里，外部可追加：

```go
import "github.com/dafsic/webx/modules/logs"

fx.Supply(fx.Annotate(
    logs.WithSource(true), // 开启 source 字段
    fx.ResultTags(`group:"`+logs.OptionsGroup+`"`),
))
```

可用选项：

| 选项 | 说明 |
| --- | --- |
| `WithLevel(string)` | 字符串级别，等价于 `--log-level` |
| `WithLevelVar(*slog.LevelVar)` | 复用外部 LevelVar |
| `WithFormat(string)` | `text` / `json` |
| `WithWriter(io.Writer)` | 切换输出目标（默认 `os.Stdout`） |
| `WithSource(bool)` | 是否记录调用方文件/行 |

后置的 option 会覆盖前置的，因此外部 `fx.Supply` 的配置优先级 ≥ CLI flag（具体由 fx 交付顺序决定）。需要严格优先级时直接使用 `WithLevelVar` 等强覆盖型选项。

## panic 级别

slog 没有 `Panic` 级别。模块定义 `LevelPanic slog.Level = 12`（位于 `Error` 之上），仅用于"过滤阈值"语义。需要"记录后直接 panic"时使用：

```go
logs.Panic(l, "unrecoverable", "err", err)
```

它会以 `LevelPanic` 写入一条记录，随后 `panic(msg)`。

## 文件组织

- `comm.go` — 常量与默认值。
- `options.go` — `Config` 与函数式选项。
- `module.go` — `app.Module` 实现与 slog handler 构造。
