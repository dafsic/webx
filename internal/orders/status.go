package main

import ordersv1 "github.com/dafsic/webx/proto_go/orders/v1"

// statusToString maps a proto OrderStatus to its database string value.
var statusToString = map[ordersv1.OrderStatus]string{
	ordersv1.OrderStatus_ORDER_STATUS_PENDING:   "pending",
	ordersv1.OrderStatus_ORDER_STATUS_PAID:      "paid",
	ordersv1.OrderStatus_ORDER_STATUS_PREPARING: "preparing",
	ordersv1.OrderStatus_ORDER_STATUS_SERVED:    "served",
	ordersv1.OrderStatus_ORDER_STATUS_CANCELLED: "cancelled",
}

// stringToStatus is the inverse of statusToString.
var stringToStatus = func() map[string]ordersv1.OrderStatus {
	m := make(map[string]ordersv1.OrderStatus, len(statusToString))
	for k, v := range statusToString {
		m[v] = k
	}
	return m
}()

// statusString returns the database string for a proto status, or "" when the
// status is unspecified/unknown.
func statusString(s ordersv1.OrderStatus) string { return statusToString[s] }

// statusFromString maps a database string back to a proto status.
func statusFromString(s string) ordersv1.OrderStatus {
	if v, ok := stringToStatus[s]; ok {
		return v
	}
	return ordersv1.OrderStatus_ORDER_STATUS_UNSPECIFIED
}
