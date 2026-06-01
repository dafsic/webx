package main

import (
	"context"
	"fmt"

	catalogv1 "github.com/dafsic/webx/proto_go/catalog/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// catalogClient is a thin gRPC client used to resolve menu item details when an
// order is created.
type catalogClient struct {
	conn *grpc.ClientConn
	cli  catalogv1.CatalogServiceClient
}

// newCatalogClient dials the catalog service. The connection is lazy; dialing
// errors surface on the first RPC.
func newCatalogClient(addr string) (*catalogClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("orders: dial catalog %s: %w", addr, err)
	}
	return &catalogClient{conn: conn, cli: catalogv1.NewCatalogServiceClient(conn)}, nil
}

// GetItem fetches a single menu item from the catalog service.
func (c *catalogClient) GetItem(ctx context.Context, id int64) (*catalogv1.MenuItem, error) {
	resp, err := c.cli.GetItem(ctx, &catalogv1.GetItemRequest{Id: id})
	if err != nil {
		return nil, err
	}
	return resp.GetItem(), nil
}

// Close releases the underlying connection.
func (c *catalogClient) Close() error { return c.conn.Close() }
