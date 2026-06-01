package main

import (
	"context"
	"log/slog"
	"strings"

	"github.com/dafsic/webx/internal/catalog/model"
	catalogv1 "github.com/dafsic/webx/proto_go/catalog/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// server implements catalogv1.CatalogServiceServer.
type server struct {
	catalogv1.UnimplementedCatalogServiceServer
	repo   *Repository
	logger *slog.Logger
}

// NewServer constructs the gRPC service implementation.
func NewServer(repo *Repository, logger *slog.Logger) catalogv1.CatalogServiceServer {
	return &server{repo: repo, logger: logger}
}

// ListItems returns the menu, optionally filtered to available items.
func (s *server) ListItems(ctx context.Context, req *catalogv1.ListItemsRequest) (*catalogv1.ListItemsResponse, error) {
	items, err := s.repo.List(ctx, req.GetAvailableOnly())
	if err != nil {
		s.logger.Error("catalog: list items", "err", err)
		return nil, status.Error(codes.Internal, "failed to list items")
	}
	out := make([]*catalogv1.MenuItem, 0, len(items))
	for i := range items {
		out = append(out, toProto(&items[i]))
	}
	return &catalogv1.ListItemsResponse{Items: out}, nil
}

// GetItem returns a single menu item by id.
func (s *server) GetItem(ctx context.Context, req *catalogv1.GetItemRequest) (*catalogv1.GetItemResponse, error) {
	if req.GetId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	item, err := s.repo.Get(ctx, req.GetId())
	if err != nil {
		s.logger.Error("catalog: get item", "id", req.GetId(), "err", err)
		return nil, status.Error(codes.Internal, "failed to get item")
	}
	if item == nil {
		return nil, status.Error(codes.NotFound, "item not found")
	}
	return &catalogv1.GetItemResponse{Item: toProto(item)}, nil
}

// CreateItem adds a new menu item.
func (s *server) CreateItem(ctx context.Context, req *catalogv1.CreateItemRequest) (*catalogv1.CreateItemResponse, error) {
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.GetPriceCents() < 0 {
		return nil, status.Error(codes.InvalidArgument, "price_cents must be non-negative")
	}
	desc := req.GetDescription()
	price := req.GetPriceCents()
	available := req.GetAvailable()
	created, err := s.repo.Create(ctx, model.MenuItem{
		Name:        &name,
		Description: &desc,
		PriceCents:  &price,
		Available:   &available,
	})
	if err != nil {
		s.logger.Error("catalog: create item", "err", err)
		return nil, status.Error(codes.Internal, "failed to create item")
	}
	return &catalogv1.CreateItemResponse{Item: toProto(created)}, nil
}

// UpdateItem overwrites an existing menu item (full replacement of editable
// fields).
func (s *server) UpdateItem(ctx context.Context, req *catalogv1.UpdateItemRequest) (*catalogv1.UpdateItemResponse, error) {
	if req.GetId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.GetPriceCents() < 0 {
		return nil, status.Error(codes.InvalidArgument, "price_cents must be non-negative")
	}
	desc := req.GetDescription()
	price := req.GetPriceCents()
	available := req.GetAvailable()
	updated, err := s.repo.Update(ctx, req.GetId(), model.MenuItem{
		Name:        &name,
		Description: &desc,
		PriceCents:  &price,
		Available:   &available,
	})
	if err != nil {
		s.logger.Error("catalog: update item", "id", req.GetId(), "err", err)
		return nil, status.Error(codes.Internal, "failed to update item")
	}
	if updated == nil {
		return nil, status.Error(codes.NotFound, "item not found")
	}
	return &catalogv1.UpdateItemResponse{Item: toProto(updated)}, nil
}

// DeleteItem removes a menu item.
func (s *server) DeleteItem(ctx context.Context, req *catalogv1.DeleteItemRequest) (*catalogv1.DeleteItemResponse, error) {
	if req.GetId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	ok, err := s.repo.Delete(ctx, req.GetId())
	if err != nil {
		s.logger.Error("catalog: delete item", "id", req.GetId(), "err", err)
		return nil, status.Error(codes.Internal, "failed to delete item")
	}
	if !ok {
		return nil, status.Error(codes.NotFound, "item not found")
	}
	return &catalogv1.DeleteItemResponse{}, nil
}

// toProto maps a model.MenuItem to the wire MenuItem message.
func toProto(m *model.MenuItem) *catalogv1.MenuItem {
	out := &catalogv1.MenuItem{
		Id:          derefInt64(m.ID),
		Name:        derefString(m.Name),
		Description: derefString(m.Description),
		PriceCents:  derefInt64(m.PriceCents),
		Available:   derefBool(m.Available),
	}
	if m.CreatedAt != nil {
		out.CreatedAt = m.CreatedAt.Unix()
	}
	if m.UpdatedAt != nil {
		out.UpdatedAt = m.UpdatedAt.Unix()
	}
	return out
}

func derefInt64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func derefBool(p *bool) bool {
	if p == nil {
		return false
	}
	return *p
}
