// Package gateway holds the HTTP gateway in charge of receiving REST traffic
// from the outside world, validating JWTs and forwarding the calls to the
// internal gRPC microservices via the grpc-gateway runtime.
package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/dafsic/webx/app"
	"github.com/dafsic/webx/modules/grpcclient"
	"github.com/dafsic/webx/pkg/authmd"
	peoplev1 "github.com/dafsic/webx/proto_go/people/v1"
	"github.com/gin-gonic/gin"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/urfave/cli/v2"
	"go.uber.org/fx"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Flag identifiers for the gateway module.
const (
	ModuleName = "gateway"

	DefaultPeopleAddr  = "people:50051"
	DefaultJWTTTL      = 24 * time.Hour
	DefaultJWTIssuer   = "webx-people"
	DefaultSwaggerFile = "proto_go/webx.swagger.json"
)

// Module wires the gateway business logic onto the gin engine provided by
// modules/ginserver. It does NOT own the HTTP server itself.
type Module struct{}

// New returns a Module.
func New() *Module { return &Module{} }

// Name implements app.Module.
func (m *Module) Name() string { return ModuleName }

// Configure implements app.Module.
func (m *Module) Configure(a *cli.App) {
	a.Flags = append(a.Flags,
		&cli.StringFlag{Name: "people-grpc-addr", Value: DefaultPeopleAddr, Usage: "people service gRPC target"},
		&cli.StringFlag{Name: "jwt-secret", Usage: "JWT secret (must match people service)"},
		&cli.DurationFlag{Name: "jwt-ttl", Value: DefaultJWTTTL, Usage: "Expected token TTL (used for refresh policy)"},
		&cli.StringFlag{Name: "jwt-issuer", Value: DefaultJWTIssuer, Usage: "Expected token issuer"},
		&cli.BoolFlag{Name: "swagger-enable", Value: true, Usage: "Expose /swagger UI + /swagger/swagger.json"},
		&cli.StringFlag{Name: "swagger-file", Value: DefaultSwaggerFile, Usage: "Path to merged openapi swagger.json"},
	)
}

// Install implements app.Module.
func (m *Module) Install(ctx app.Context) fx.Option {
	peopleAddr := ctx.String("people-grpc-addr")
	secret := ctx.String("jwt-secret")
	ttl := ctx.Duration("jwt-ttl")
	issuer := ctx.String("jwt-issuer")
	swaggerEnable := ctx.Bool("swagger-enable")
	swaggerFile := ctx.String("swagger-file")

	return fx.Options(
		fx.Provide(func() (*authmd.Signer, error) { return authmd.New(secret, ttl, issuer) }),
		fx.Invoke(func(lc fx.Lifecycle, r *gin.Engine, d *grpcclient.Dialer,
			signer *authmd.Signer, logger *slog.Logger) error {

			startCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			peopleCC, err := d.Dial(startCtx, peopleAddr)
			if err != nil {
				return fmt.Errorf("gateway: dial people: %w", err)
			}

			mux := runtime.NewServeMux(
				runtime.WithMetadata(forwardAuthMetadata(signer)),
				runtime.WithErrorHandler(errorHandler),
			)
			if err := peoplev1.RegisterPeopleServiceHandler(startCtx, mux, peopleCC); err != nil {
				return fmt.Errorf("gateway: register people: %w", err)
			}

			r.GET("/healthz", func(c *gin.Context) { c.Status(http.StatusNoContent) })
			r.Any("/api/*p", gin.WrapH(mux))

			if swaggerEnable {
				registerSwagger(r, swaggerFile)
				logger.Info("gateway: swagger enabled", "path", "/swagger", "spec", swaggerFile)
			}

			lc.Append(fx.Hook{
				OnStop: func(_ context.Context) error { return peopleCC.Close() },
			})
			return nil
		}),
	)
}

// forwardAuthMetadata is invoked by grpc-gateway runtime for every request:
// we parse the bearer token (when present) and put uid/jti onto outgoing
// metadata. Missing/invalid tokens simply don't get metadata; downstream
// services that need auth can return Unauthenticated themselves.
func forwardAuthMetadata(signer *authmd.Signer) func(context.Context, *http.Request) metadata.MD {
	return func(ctx context.Context, r *http.Request) metadata.MD {
		md := metadata.MD{}
		if rid, ok := ctx.Value(ginRequestIDKey{}).(string); ok && rid != "" {
			md.Set("x-request-id", rid)
		}
		h := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(h, prefix) {
			return md
		}
		claims, err := signer.Parse(strings.TrimPrefix(h, prefix))
		if err != nil {
			return md
		}
		md.Set(authmd.MDUserID, fmt.Sprintf("%d", claims.UserID))
		md.Set(authmd.MDJTI, claims.ID)
		return md
	}
}

type ginRequestIDKey struct{}

// errorHandler converts a gRPC status error to a JSON HTTP response with the
// same shape every endpoint uses.
func errorHandler(ctx context.Context, mux *runtime.ServeMux, marshaler runtime.Marshaler, w http.ResponseWriter, r *http.Request, err error) {
	st, _ := status.FromError(err)
	code := runtime.HTTPStatusFromCode(st.Code())
	body := map[string]any{
		"code":    st.Code().String(),
		"message": st.Message(),
	}
	if details := st.Details(); len(details) > 0 {
		body["details"] = details
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = marshaler.NewEncoder(w).Encode(body)
}

// registerSwagger mounts the swagger spec at /swagger/swagger.json and a
// simple HTML at /swagger that loads swagger-ui from a public CDN.
func registerSwagger(r *gin.Engine, specPath string) {
	r.StaticFile("/swagger/swagger.json", specPath)
	r.GET("/swagger", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(swaggerHTML))
	})
}

const swaggerHTML = `<!DOCTYPE html>
<html><head><title>Webx API</title>
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui.css">
</head><body>
<div id="swagger-ui"></div>
<script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
<script>
window.onload = () => SwaggerUIBundle({url: "/swagger/swagger.json", dom_id: "#swagger-ui"});
</script></body></html>`
