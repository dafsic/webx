# redis

基于 [`github.com/redis/go-redis/v9`](https://github.com/redis/go-redis) 的 Redis 模块。统一通过 `redis.UniversalClient` 暴露客户端，根据配置自动适配 **单实例 / 哨兵 / 集群** 三种模式。

## 特性

- 一套配置覆盖三种部署模式，由 `go-redis` 的 `NewUniversalClient` 选择。
- 启动期 `PING` 探测，连不上直接 fail-fast。
- 退出时自动 `Close()`。
- CLI flag + 环境变量 + 函数式选项 + `fx` group 四种配置入口。

## 模式选择规则

由 `go-redis` 决定，遵循以下优先级：

| 条件 | 模式 |
| --- | --- |
| `--redis-master-name` 非空 | Sentinel / Failover |
| `--redis-addrs` 多个地址 | Cluster |
| 其它 | 单实例 |

## CLI

| Flag | Env | 默认 | 说明 |
| --- | --- | --- | --- |
| `--redis-addrs` | `REDIS_ADDRS` | `127.0.0.1:6379` | 地址列表，逗号分隔 |
| `--redis-db` | `REDIS_DB` | `0` | DB 索引（仅单实例） |
| `--redis-username` | `REDIS_USERNAME` | | ACL 用户名 (Redis 6+) |
| `--redis-password` | `REDIS_PASSWORD` | | 密码 |
| `--redis-master-name` | `REDIS_MASTER_NAME` | | 哨兵 master 名 |
| `--redis-pool-size` | `REDIS_POOL_SIZE` | `10` | 连接池大小 |
| `--redis-dial-timeout` | `REDIS_DIAL_TIMEOUT` | `5s` | 拨号超时 |
| `--redis-read-timeout` | `REDIS_READ_TIMEOUT` | `3s` | 读超时 |
| `--redis-write-timeout` | `REDIS_WRITE_TIMEOUT` | `3s` | 写超时 |

## 快速开始

```go
package main

import (
    "log"

    "github.com/dafsic/webx/app"
    "github.com/dafsic/webx/modules/logs"
    "github.com/dafsic/webx/modules/redis"
)

func main() {
    a := app.NewApplication("myservice", "demo")
    a.Install(logs.New(), redis.New())
    if err := a.Run(); err != nil {
        log.Fatal(err)
    }
}
```

```sh
# 单实例
./myservice --redis-addrs=127.0.0.1:6379 --redis-db=1

# 集群
./myservice --redis-addrs=10.0.0.1:6379,10.0.0.2:6379,10.0.0.3:6379

# 哨兵
./myservice --redis-addrs=10.0.0.1:26379,10.0.0.2:26379 \
            --redis-master-name=mymaster

# 通过环境变量
REDIS_ADDRS=10.0.0.1:6379 REDIS_PASSWORD=secret ./myservice
```

## 在其它模块中使用

`redis.Module` 提供以下类型：

| 类型 | 说明 |
| --- | --- |
| `redis.UniversalClient` | 主客户端，命令调用入口 |
| `*redis.Config` | 已解析配置 |

```go
import (
    "context"

    goredis "github.com/redis/go-redis/v9"
    "go.uber.org/fx"
)

fx.Invoke(func(rc goredis.UniversalClient) error {
    return rc.Set(context.Background(), "key", "value", 0).Err()
})
```

## 扩展配置

模块用 `fx` group `redis.OptionsGroup`（值 `"redis_options"`）收集所有 `redis.Option`，CLI flag 派生的 options 也走这个 group。外部可以追加或覆盖：

```go
import "github.com/dafsic/webx/modules/redis"

fx.Supply(fx.Annotate(
    redis.WithPoolSize(64),
    fx.ResultTags(`group:"`+redis.OptionsGroup+`"`),
))
```

可用选项：

| 选项 | 说明 |
| --- | --- |
| `WithAddrs(...string)` | 设置地址列表 |
| `WithAddrsCSV(string)` | 逗号分隔字符串，等价于 `--redis-addrs` |
| `WithDB(int)` | DB 索引 |
| `WithUsername(string)` | ACL 用户名 |
| `WithPassword(string)` | 密码 |
| `WithMasterName(string)` | 启用哨兵模式 |
| `WithPoolSize(int)` | 连接池大小 |
| `WithDialTimeout(time.Duration)` | 拨号超时 |
| `WithReadTimeout(time.Duration)` | 读超时 |
| `WithWriteTimeout(time.Duration)` | 写超时 |
| `WithPingTimeout(time.Duration)` | 启动期 `PING` 超时；`0` 跳过探测 |

## 生命周期

- **OnStart**：在 `PingTimeout`（默认 5s）内执行 `PING`，失败则 `Close()` 并返回错误，从 `Application.Run()` 向上传播。
- **OnStop**：调用 `client.Close()`。

需要禁用启动探测时：

```go
fx.Supply(fx.Annotate(
    redis.WithPingTimeout(0),
    fx.ResultTags(`group:"`+redis.OptionsGroup+`"`),
))
```

## 文件组织

- `comm.go` — 常量与默认值
- `options.go` — `Config` 与函数式选项
- `module.go` — `app.Module` 实现与客户端构造
