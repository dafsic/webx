// Package grpcerr provides helpers to convert Go errors into gRPC
// status.Status values that grpc-gateway can translate to HTTP responses
// the front-end expects. Business code returns these from gRPC handlers; the
// gateway's WithErrorHandler renders them to JSON.
package grpcerr

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Sentinel errors business layers may return. Wrap them with fmt.Errorf("...:
// %w", err) and the helpers below will pick up the right code.
var (
	ErrInvalidArgument = errors.New("invalid argument")
	ErrUnauthenticated = errors.New("unauthenticated")
	ErrPermissionDenied = errors.New("permission denied")
	ErrNotFound        = errors.New("not found")
	ErrAlreadyExists   = errors.New("already exists")
	ErrInternal        = errors.New("internal")
)

// Wrap converts a Go error into a *status.Status. Already-statused errors
// are returned unchanged.
func Wrap(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := status.FromError(err); ok {
		return err
	}
	switch {
	case errors.Is(err, ErrInvalidArgument):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, ErrUnauthenticated):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, ErrPermissionDenied):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}

// Code returns the gRPC code for err, defaulting to Internal.
func Code(err error) codes.Code {
	if s, ok := status.FromError(err); ok {
		return s.Code()
	}
	return codes.Internal
}
