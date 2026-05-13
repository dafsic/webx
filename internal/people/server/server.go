// Package server is the gRPC adapter for the people service. It implements
// peoplev1.PeopleServiceServer by delegating to use cases under
// internal/people/api and registers itself with the shared *grpc.Server
// provided by modules/grpcserver.
package server

import (
	"context"

	"github.com/dafsic/webx/app"
	"github.com/dafsic/webx/internal/people/api"
	"github.com/dafsic/webx/pkg/grpcerr"
	peoplev1 "github.com/dafsic/webx/proto_go/people/v1"
	"github.com/urfave/cli/v2"
	"go.uber.org/fx"
	"google.golang.org/grpc"
)

// ModuleName is the fx.Module name used for this adapter.
const ModuleName = "people-grpc"

// Module wires the gRPC adapter into the grpcserver provided by the
// framework.
type Module struct{}

// New returns a Module.
func New() *Module { return &Module{} }

// Name implements app.Module.
func (m *Module) Name() string { return ModuleName }

// Configure implements app.Module.
func (m *Module) Configure(*cli.App) {}

// Install implements app.Module.
func (m *Module) Install(_ app.Context) fx.Option {
	return fx.Options(
		fx.Provide(newPeopleServer),
		fx.Invoke(func(s *grpc.Server, ps *peopleServer) {
			peoplev1.RegisterPeopleServiceServer(s, ps)
		}),
	)
}

type peopleServer struct {
	peoplev1.UnimplementedPeopleServiceServer
	login *api.LoginService
}

func newPeopleServer(login *api.LoginService) *peopleServer {
	return &peopleServer{login: login}
}

// Login implements peoplev1.PeopleServiceServer.
func (s *peopleServer) Login(ctx context.Context, req *peoplev1.LoginRequest) (*peoplev1.LoginResponse, error) {
	if req == nil {
		req = &peoplev1.LoginRequest{}
	}
	res, err := s.login.Login(ctx, req.GetUsername(), req.GetPassword())
	if err != nil {
		return nil, grpcerr.Wrap(err)
	}
	return &peoplev1.LoginResponse{
		Token:     res.Token,
		ExpiresAt: res.ExpiresAt.Unix(),
		User: &peoplev1.User{
			Id:        res.UserID,
			Username:  res.Username,
			Nickname:  res.Nickname,
			CreatedAt: res.CreatedAt.Unix(),
			UpdatedAt: res.UpdatedAt.Unix(),
		},
	}, nil
}
