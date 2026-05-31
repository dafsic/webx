# gateway

webx 平台的公共 HTTP 入口，基于 [`grpc-gateway`](https://github.com/grpc-ecosystem/grpc-gateway) 的反向代理。

浏览器 / 外部客户端用 HTTP/JSON 访问本服务，网关把每个请求翻译成 gRPC 调用转发给后端微服务（如 people）。REST 路由映射与 OpenAPI 文档都来自 `.proto` 定义，新增或修改接口只需重新生成 `proto_go`。

```
前端 / 客户端  --HTTP/JSON-->  gateway(:8080)  --gRPC-->  people(:50051)
```

## 特性

- 路由、参数映射、在线文档全部由 protobuf 自动生成，无需手写 REST handler。
- **集中鉴权中间层**：对受保护路由统一做 JWT 验签 + 过期校验，无效 token 在网关边缘即返回 `401`，不再打到后端。
- 自动透传 `Authorization: Bearer <jwt>` 到 gRPC metadata，配合各服务的鉴权拦截器即可生效。
- 内置可配置的 CORS 白名单（`*` 放行任意来源，此时按规范禁用凭证）。
- 内置 Swagger UI 在线文档，可在浏览器中直接调试接口。
- 与 `logs` 模块统一日志，遵循 `app.Module` + `fx` 生命周期，优雅启停。

## 端点

| 路径 | 方法 | 说明 |
| --- | --- | --- |
| `/api/v1/...` | 由 proto 注解决定 | 代理到后端 gRPC 服务的 REST 路由 |
| `/docs` | GET | Swagger UI 在线文档 |
| `/openapi.json` | GET | 生成的 OpenAPI（swagger 2.0）文档 |

当前已接入的 people 服务路由：

| 路径 | 方法 | 说明 |
| --- | --- | --- |
| `/api/v1/people:getChallenge` | POST | 获取登录挑战码（nonce） |
| `/api/v1/people:login` | POST | EIP-191 personal_sign 登录，返回 JWT |
| `/api/v1/people:logout` | POST | 登出，吊销当前 JWT |
| `/api/v1/people:checkPermission` | POST | RBAC 权限检查 |

## 依赖

网关依赖 [logs](../../modules/logs/README.md) 提供的 `*slog.Logger`，并需要后端微服务（people）可达。

## CLI

| Flag | Env | 默认 | 说明 |
| --- | --- | --- | --- |
| `--gateway-http-addr` | `GATEWAY_HTTP_ADDR` | `:8080` | HTTP 监听地址 |
| `--gateway-jwt-secret` | `GATEWAY_JWT_SECRET` | （无，必填） | 与 people 服务共享的 HS256 secret，用于网关验签 |
| `--gateway-people-addr` | `GATEWAY_PEOPLE_ADDR` | `127.0.0.1:50051` | people 服务 gRPC 地址 |
| `--gateway-openapi-spec` | `GATEWAY_OPENAPI_SPEC` | `./proto_go/webx.swagger.json` | OpenAPI swagger JSON 路径（留空则关闭 `/docs`） |
| `--gateway-cors-origins` | `GATEWAY_CORS_ORIGINS` | `*` | 逗号分隔的允许来源，或 `*` 放行任意来源 |

## 鉴权

网关内置一个**集中认证中间层**（[auth.go](auth.go)）：

- 公开路由（`people:getChallenge`、`people:login`）与文档（`/docs`、`/openapi.json`）无需 token。
- 其余路由必须携带 `Authorization: Bearer <jwt>`，网关用共享 secret 校验 HS256 签名与过期时间，失败返回 `401`（gRPC code 16）。
- 网关**只做认证**；具体的 RBAC 权限判断仍由各微服务自己的拦截器负责。token 校验通过后原样透传给后端。
- secret 必须与 people 服务的 `--people-jwt-secret` 保持一致，否则验签必然失败。

## 快速开始

启动网关（需要 people 服务已在 `127.0.0.1:50051` 运行）：

```sh
# 终端 1：people 微服务（依赖 postgres + redis）
go run ./internal/people --people-jwt-secret <secret>

# 终端 2：网关
go run ./internal/gateway --gateway-jwt-secret <secret>
```

浏览器打开 <http://localhost:8080/docs> 即可查看并调试接口。

调用示例：

```sh
# 1. 获取挑战码
curl -s -X POST http://localhost:8080/api/v1/people:getChallenge \
  -H 'Content-Type: application/json' \
  -d '{"address":"0xYourWalletAddress"}'

# 2. 用钱包对返回的 message 做 personal_sign，再带签名登录
curl -s -X POST http://localhost:8080/api/v1/people:login \
  -H 'Content-Type: application/json' \
  -d '{"address":"0xYourWalletAddress","signature":"0x..."}'

# 3. 携带返回的 JWT 调用受保护接口
curl -s -X POST http://localhost:8080/api/v1/people:checkPermission \
  -H 'Authorization: Bearer <jwt>' \
  -H 'Content-Type: application/json' \
  -d '{"resource":"permission","action":"read"}'
```

## 新增微服务

在 [gateway.go](gateway.go) 的 `buildHandler` 中追加一行注册即可（搜索 `Add new services here`）：

```go
if err := xxxv1.RegisterXxxServiceHandlerFromEndpoint(
    ctx, gwMux, cfg.XxxAddr, dialOpts,
); err != nil {
    return nil, fmt.Errorf("gateway: register xxx handler: %w", err)
}
```

并为该服务的地址增加对应的 CLI flag 与 `Config` 字段。
