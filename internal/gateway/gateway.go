package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	peoplev1 "github.com/dafsic/webx/proto_go/people/v1"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// swaggerUIHTML is a minimal Swagger UI page (loaded from the public CDN) that
// renders the OpenAPI spec served at /openapi.json.
const swaggerUIHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Webx API Docs</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.onload = () => {
      window.ui = SwaggerUIBundle({
        url: "/openapi.json",
        dom_id: "#swagger-ui",
      });
    };
  </script>
</body>
</html>`

// buildHandler constructs the gateway's root HTTP handler: a grpc-gateway
// ServeMux that reverse-proxies REST calls to the backing microservices, plus
// the OpenAPI docs endpoints, all wrapped in CORS.
func buildHandler(ctx context.Context, cfg *Config, logger *slog.Logger) (http.Handler, error) {
	gwMux := runtime.NewServeMux()

	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	// Register each microservice's gateway handler. Add new services here.
	if err := peoplev1.RegisterPeopleServiceHandlerFromEndpoint(
		ctx, gwMux, cfg.PeopleAddr, dialOpts,
	); err != nil {
		return nil, fmt.Errorf("gateway: register people handler: %w", err)
	}

	mux := http.NewServeMux()
	// Protected API routes go through the centralized auth layer; the more
	// specific /docs and /openapi.json patterns registered below bypass it.
	mux.Handle("/", authMiddleware([]byte(cfg.JWTSecret), gwMux))
	registerDocs(mux, cfg, logger)

	return corsMiddleware(cfg.CORSOrigins, mux), nil
}

// registerDocs serves the OpenAPI spec and a Swagger UI page when the spec file
// exists. A missing spec is logged but non-fatal.
func registerDocs(mux *http.ServeMux, cfg *Config, logger *slog.Logger) {
	if cfg.OpenAPISpec == "" {
		return
	}
	spec, err := os.ReadFile(cfg.OpenAPISpec)
	if err != nil {
		logger.Warn("gateway: OpenAPI spec not found, docs disabled",
			"path", cfg.OpenAPISpec, "err", err)
		return
	}

	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(spec)
	})
	mux.HandleFunc("/docs", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(swaggerUIHTML))
	})
	logger.Info("gateway: API docs enabled", "ui", "/docs", "spec", "/openapi.json")
}
