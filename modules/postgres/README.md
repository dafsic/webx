# postgres

基于 [`sqlx`](https://github.com/jmoiron/sqlx) + [`sql-migrate`](https://github.com/rubenv/sql-migrate) + [`squirrel`](https://github.com/Masterminds/squirrel) + [`pgx/v5/stdlib`](https://github.com/jackc/pgx) 的 PostgreSQL 模块。

## 特性

- 固定使用 `pgx/v5/stdlib` 驱动（`database/sql` 注册名 `pgx`），不暴露 driver 选择。
- 提供 `Database` 接口，封装 `Ping / Session / Transact / TransactReadOnly / Close`；事务在 panic 或 error 时自动回滚。
- 内置 `Migrator`，支持启动期 `up` / `down` 自动迁移。
- 内置 squirrel 构建器，PostgreSQL `$N` 占位符已预配置。
- 启动期 `PING` 探测，连不上即 fail-fast；退出时自动 `Close()`。
- CLI flag + 环境变量 + 函数式选项 + `fx` group 四种配置入口。

## 依赖

模块依赖 [logs](../logs/README.md) 提供的 `*slog.Logger`。使用时务必同时安装：

```go
a.Install(logs.New(), postgres.New())
```

## CLI

| Flag | Env | 默认 | 说明 |
| --- | --- | --- | --- |
| `--postgres-dsn` | `POSTGRES_DSN` | `host=localhost port=5432 user=postgres password=postgres dbname=postgres sslmode=disable` | 连接串 |
| `--postgres-max-open-conns` | `POSTGRES_MAX_OPEN_CONNS` | `25` | 最大连接数 |
| `--postgres-max-idle-conns` | `POSTGRES_MAX_IDLE_CONNS` | `5` | 空闲连接数 |
| `--postgres-conn-max-lifetime` | `POSTGRES_CONN_MAX_LIFETIME` | `30m` | 连接最大寿命 |
| `--postgres-conn-max-idle-time` | `POSTGRES_CONN_MAX_IDLE_TIME` | `5m` | 连接最大空闲 |
| `--postgres-ping-timeout` | `POSTGRES_PING_TIMEOUT` | `5s` | 启动 `PING` 超时（`0` 跳过） |
| `--postgres-migrate` | `POSTGRES_MIGRATE` | `off` | 启动迁移模式：`off` / `up` / `down` |
| `--postgres-migrate-dir` | `POSTGRES_MIGRATE_DIR` | `./migrations` | 迁移文件目录 |
| `--postgres-migrate-table` | `POSTGRES_MIGRATE_TABLE` | `migrations` | 迁移记录表 |
| `--postgres-migrate-schema` | `POSTGRES_MIGRATE_SCHEMA` | `public` | 迁移记录表 schema |

## 快速开始

```go
package main

import (
    "log"

    "github.com/dafsic/webx/app"
    "github.com/dafsic/webx/modules/logs"
    "github.com/dafsic/webx/modules/postgres"
)

func main() {
    a := app.NewApplication("myservice", "demo")
    a.Install(logs.New(), postgres.New())
    if err := a.Run(); err != nil {
        log.Fatal(err)
    }
}
```

```sh
# 仅连接，不迁移
./myservice --postgres-dsn="postgres://user:pass@127.0.0.1:5432/app?sslmode=disable"

# 启动时执行 up
./myservice --postgres-migrate=up --postgres-migrate-dir=./migrations

# 回滚最近一步
./myservice --postgres-migrate=down

# 环境变量
POSTGRES_DSN=... POSTGRES_MIGRATE=up ./myservice
```

## fx 提供的类型

| 类型 | 用途 |
| --- | --- |
| `postgres.Database` | 高层封装（推荐） |
| `*sqlx.DB` | 底层句柄，需要原生 sqlx 操作时使用 |
| `*postgres.Migrator` | 运行时手动触发迁移 |
| `*postgres.Config` | 已解析配置 |

```go
import (
    "context"

    "github.com/dafsic/webx/modules/postgres"
    "go.uber.org/fx"
)

fx.Invoke(func(db postgres.Database) error {
    return db.Transact(context.Background(), func(tx *sqlx.Tx) error {
        _, err := tx.Exec("INSERT INTO users(name) VALUES ($1)", "alice")
        return err
    })
})
```

## SQL 构建（squirrel）

模块的 `sql.go` 暴露了一组 PostgreSQL 预配置的 squirrel 入口：

```go
import (
    "github.com/dafsic/webx/modules/postgres"
    sq "github.com/Masterminds/squirrel"
)

q, args, err := postgres.ToSQL(
    postgres.Select("id", "name").
        From("users").
        Where(sq.Eq{"active": true}).
        OrderBy("id DESC").
        Limit(10),
)
// q = `SELECT id, name FROM users WHERE active = $1 ORDER BY id DESC LIMIT 10`
```

可用函数：`Select / Insert / Update / Delete / ToSQL`，以及 `Builder`（裸 `sq.StatementBuilderType`，占位符已设置为 `sq.Dollar`）。

## 事务

```go
err := db.Transact(ctx, func(tx *sqlx.Tx) error {
    if _, err := tx.Exec("..."); err != nil {
        return err
    }
    if _, err := tx.Exec("..."); err != nil {
        return err
    }
    return nil
})
```

- `txFunc` 返回 error → 自动 `Rollback`。
- `txFunc` panic → 自动 `Rollback` 后重新 panic。
- 正常返回 → 自动 `Commit`，commit 失败的错误向外返回。
- 只读事务：`db.TransactReadOnly(ctx, ...)`。

## 迁移

迁移文件遵循 [sql-migrate](https://github.com/rubenv/sql-migrate) 的格式，命名建议 `Vnnn__name.sql`：

```sql
-- migrations/V001__init.sql

-- +migrate Up
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL
);

-- +migrate Down
DROP TABLE users;
```

启动期：

| 模式 | 行为 |
| --- | --- |
| `off`（默认） | 不迁移 |
| `up` | 应用所有 pending 迁移 |
| `down` | 回滚最近一条已应用迁移 |

启动期迁移失败会关闭连接池并把错误返回给 `fx`，进程退出。

代码中手动触发：

```go
fx.Invoke(func(m *postgres.Migrator) error {
    _, err := m.Up()
    return err
})
```

可用方法：`Up() / Down() / Status() ([]*migrate.MigrationRecord, error)`。

## 扩展配置

使用 `fx` group `postgres.OptionsGroup`（值 `"postgres_options"`）追加 `postgres.Option`：

```go
fx.Supply(fx.Annotate(
    postgres.WithMaxOpenConns(100),
    fx.ResultTags(`group:"`+postgres.OptionsGroup+`"`),
))
```

可用选项：

| 选项 | 说明 |
| --- | --- |
| `WithDSN(string)` | 连接串 |
| `WithMaxOpenConns(int)` | 最大连接数 |
| `WithMaxIdleConns(int)` | 空闲连接数 |
| `WithConnMaxLifetime(time.Duration)` | 连接寿命 |
| `WithConnMaxIdleTime(time.Duration)` | 连接空闲时间 |
| `WithPingTimeout(time.Duration)` | 启动 ping 超时；`0` 跳过 |
| `WithMigrate(string)` | `off` / `up` / `down` |
| `WithMigrateDir(string)` | 迁移目录 |
| `WithMigrateTable(string)` | 迁移记录表 |
| `WithMigrateSchema(string)` | 迁移记录表 schema |

## 生命周期

- **OnStart**：`Ping`（受 `PingTimeout` 约束）→ 根据 `--postgres-migrate` 执行 `Up` / `Down` / 跳过。任一失败即关闭连接池并返回错误。
- **OnStop**：`Close()`。

## 文件组织

- `comm.go` — 常量与默认值
- `options.go` — `Config` 与函数式选项
- `database.go` — `Database` 接口与实现
- `sql.go` — squirrel 构建器辅助函数
- `migrator.go` — sql-migrate 封装
- `module.go` — `app.Module` 实现 + fx 生命周期接线
