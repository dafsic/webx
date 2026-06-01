package main

import (
	"context"
	"log/slog"
	"strings"

	"github.com/dafsic/webx/internal/orders/model"
	ordersv1 "github.com/dafsic/webx/proto_go/orders/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// server implements ordersv1.OrderServiceServer.
type server struct {
	ordersv1.UnimplementedOrderServiceServer
	repo    *Repository
	catalog *catalogClient
	logger  *slog.Logger
}

// NewServer constructs the gRPC service implementation.
func NewServer(repo *Repository, catalog *catalogClient, logger *slog.Logger) ordersv1.OrderServiceServer {
	return &server{repo: repo, catalog: catalog, logger: logger}
}

// CreateOrder places a new order, resolving each line's price and name from the
// catalog service and computing the total server-side.
func (s *server) CreateOrder(ctx context.Context, req *ordersv1.CreateOrderRequest) (*ordersv1.CreateOrderResponse, error) {
	tableNo := strings.TrimSpace(req.GetTableNo())
	if tableNo == "" {
		return nil, status.Error(codes.InvalidArgument, "table_no is required")
	}
	if len(req.GetItems()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "at least one item is required")
	}

	var (
		total int64
		items []model.OrderItem
	)
	for _, line := range req.GetItems() {
		if line.GetMenuItemId() <= 0 {
			return nil, status.Error(codes.InvalidArgument, "menu_item_id is required")
		}
		if line.GetQuantity() <= 0 {
			return nil, status.Error(codes.InvalidArgument, "quantity must be positive")
		}
		menuItem, err := s.catalog.GetItem(ctx, line.GetMenuItemId())
		if err != nil {
			if status.Code(err) == codes.NotFound {
				return nil, status.Errorf(codes.InvalidArgument, "menu item %d not found", line.GetMenuItemId())
			}
			s.logger.Error("orders: resolve menu item", "id", line.GetMenuItemId(), "err", err)
			return nil, status.Error(codes.Internal, "failed to resolve menu item")
		}
		if !menuItem.GetAvailable() {
			return nil, status.Errorf(codes.FailedPrecondition, "menu item %d is not available", line.GetMenuItemId())
		}

		qty := line.GetQuantity()
		unit := menuItem.GetPriceCents()
		subtotal := unit * int64(qty)
		total += subtotal

		name := menuItem.GetName()
		menuItemID := menuItem.GetId()
		items = append(items, model.OrderItem{
			MenuItemID:     &menuItemID,
			Name:           &name,
			UnitPriceCents: &unit,
			Quantity:       &qty,
			SubtotalCents:  &subtotal,
		})
	}

	pending := statusString(ordersv1.OrderStatus_ORDER_STATUS_PENDING)
	note := req.GetNote()
	order := model.Order{
		TableNo:    &tableNo,
		Status:     &pending,
		TotalCents: &total,
		Note:       &note,
	}

	storedOrder, storedItems, err := s.repo.Create(ctx, order, items)
	if err != nil {
		s.logger.Error("orders: create order", "err", err)
		return nil, status.Error(codes.Internal, "failed to create order")
	}
	return &ordersv1.CreateOrderResponse{Order: toProto(storedOrder, storedItems)}, nil
}

// GetOrder returns a single order with its items.
func (s *server) GetOrder(ctx context.Context, req *ordersv1.GetOrderRequest) (*ordersv1.GetOrderResponse, error) {
	if req.GetId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	order, items, err := s.repo.Get(ctx, req.GetId())
	if err != nil {
		s.logger.Error("orders: get order", "id", req.GetId(), "err", err)
		return nil, status.Error(codes.Internal, "failed to get order")
	}
	if order == nil {
		return nil, status.Error(codes.NotFound, "order not found")
	}
	return &ordersv1.GetOrderResponse{Order: toProto(order, items)}, nil
}

// ListOrders returns orders, optionally filtered by status.
func (s *server) ListOrders(ctx context.Context, req *ordersv1.ListOrdersRequest) (*ordersv1.ListOrdersResponse, error) {
	orders, itemsByOrder, err := s.repo.List(ctx, statusString(req.GetStatus()))
	if err != nil {
		s.logger.Error("orders: list orders", "err", err)
		return nil, status.Error(codes.Internal, "failed to list orders")
	}
	out := make([]*ordersv1.Order, 0, len(orders))
	for i := range orders {
		var items []model.OrderItem
		if orders[i].ID != nil {
			items = itemsByOrder[*orders[i].ID]
		}
		out = append(out, toProto(&orders[i], items))
	}
	return &ordersv1.ListOrdersResponse{Orders: out}, nil
}

// UpdateOrderStatus advances an order's status.
func (s *server) UpdateOrderStatus(ctx context.Context, req *ordersv1.UpdateOrderStatusRequest) (*ordersv1.UpdateOrderStatusResponse, error) {
	if req.GetId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	st := statusString(req.GetStatus())
	if st == "" {
		return nil, status.Error(codes.InvalidArgument, "a valid status is required")
	}
	order, items, err := s.repo.UpdateStatus(ctx, req.GetId(), st)
	if err != nil {
		s.logger.Error("orders: update status", "id", req.GetId(), "err", err)
		return nil, status.Error(codes.Internal, "failed to update order status")
	}
	if order == nil {
		return nil, status.Error(codes.NotFound, "order not found")
	}
	return &ordersv1.UpdateOrderStatusResponse{Order: toProto(order, items)}, nil
}

// toProto maps a model.Order plus its items to the wire Order message.
func toProto(m *model.Order, items []model.OrderItem) *ordersv1.Order {
	out := &ordersv1.Order{
		Id:         derefInt64(m.ID),
		TableNo:    derefString(m.TableNo),
		Status:     statusFromString(derefString(m.Status)),
		TotalCents: derefInt64(m.TotalCents),
		Note:       derefString(m.Note),
	}
	if m.CreatedAt != nil {
		out.CreatedAt = m.CreatedAt.Unix()
	}
	if m.UpdatedAt != nil {
		out.UpdatedAt = m.UpdatedAt.Unix()
	}
	for i := range items {
		it := &items[i]
		out.Items = append(out.Items, &ordersv1.OrderItem{
			Id:             derefInt64(it.ID),
			OrderId:        derefInt64(it.OrderID),
			MenuItemId:     derefInt64(it.MenuItemID),
			Name:           derefString(it.Name),
			UnitPriceCents: derefInt64(it.UnitPriceCents),
			Quantity:       derefInt32(it.Quantity),
			SubtotalCents:  derefInt64(it.SubtotalCents),
		})
	}
	return out
}

func derefInt64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

func derefInt32(p *int32) int32 {
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
