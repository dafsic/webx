// Package server is the gRPC adapter for the people service. It translates
// proto types and delegates all logic to api.Service.
package server

import (
"context"

"github.com/dafsic/webx/app"
"github.com/dafsic/webx/internal/people/api"
"github.com/dafsic/webx/internal/people/model"
"github.com/dafsic/webx/pkg/authmd"
"github.com/dafsic/webx/pkg/grpcerr"
peoplev1 "github.com/dafsic/webx/proto_go/people/v1"
"github.com/urfave/cli/v2"
"go.uber.org/fx"
"google.golang.org/grpc"
)

// ModuleName is the fx.Module name used for this adapter.
const ModuleName = "people-grpc"

// Module wires the gRPC adapter into the shared grpcserver.
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
	svc *api.Service
}

func newPeopleServer(svc *api.Service) *peopleServer {
	return &peopleServer{svc: svc}
}

// GetChallenge implements peoplev1.PeopleServiceServer.
func (s *peopleServer) GetChallenge(ctx context.Context, req *peoplev1.GetChallengeRequest) (*peoplev1.GetChallengeResponse, error) {
	if req == nil {
		req = &peoplev1.GetChallengeRequest{}
	}
	nonce, err := s.svc.GetChallenge(ctx, req.GetAddress())
	if err != nil {
		return nil, grpcerr.Wrap(err)
	}
	return &peoplev1.GetChallengeResponse{Nonce: nonce}, nil
}

// Login implements peoplev1.PeopleServiceServer.
func (s *peopleServer) Login(ctx context.Context, req *peoplev1.LoginRequest) (*peoplev1.LoginResponse, error) {
	if req == nil {
		req = &peoplev1.LoginRequest{}
	}
	res, err := s.svc.Login(ctx, req.GetAddress(), req.GetSignature())
	if err != nil {
		return nil, grpcerr.Wrap(err)
	}
	return &peoplev1.LoginResponse{
		Token:     res.Token,
		ExpiresAt: res.ExpiresAt.Unix(),
		IsNew:     res.IsNew,
		User: &peoplev1.User{
			Id:        res.UserID,
			Address:   res.Address,
			Nickname:  res.Nickname,
			AvatarUrl: res.AvatarURL,
			Email:     res.Email,
			Roles:     toProtoRoles(res.Roles),
			CreatedAt: res.CreatedAt.Unix(),
			UpdatedAt: res.UpdatedAt.Unix(),
		},
	}, nil
}

// Logout implements peoplev1.PeopleServiceServer.
func (s *peopleServer) Logout(ctx context.Context, _ *peoplev1.LogoutRequest) (*peoplev1.LogoutResponse, error) {
	id, ok := authmd.From(ctx)
	if !ok {
		return nil, grpcerr.Wrap(grpcerr.ErrUnauthenticated)
	}
	if err := s.svc.Logout(ctx, id.UserID, id.JTI); err != nil {
		return nil, grpcerr.Wrap(err)
	}
	return &peoplev1.LogoutResponse{}, nil
}

// CheckPermission implements peoplev1.PeopleServiceServer.
func (s *peopleServer) CheckPermission(ctx context.Context, req *peoplev1.CheckPermissionRequest) (*peoplev1.CheckPermissionResponse, error) {
	if req == nil {
		req = &peoplev1.CheckPermissionRequest{}
	}
	allowed, err := s.svc.CheckPermission(ctx, req.GetUserId(), req.GetResource(), req.GetAction())
	if err != nil {
		return nil, grpcerr.Wrap(err)
	}
	return &peoplev1.CheckPermissionResponse{Allowed: allowed}, nil
}

// ── helpers ────────────────────────────────────────────────────────────────

func toProtoRoles(roles []model.Role) []*peoplev1.Role {
	out := make([]*peoplev1.Role, len(roles))
	for i, r := range roles {
		out[i] = &peoplev1.Role{
			Id:          r.ID,
			Name:        r.Name,
			Description: r.Description,
			Permissions: toProtoPermissions(r.Permissions),
		}
	}
	return out
}

func toProtoPermissions(perms []model.Permission) []*peoplev1.Permission {
	out := make([]*peoplev1.Permission, len(perms))
	for i, p := range perms {
		out[i] = &peoplev1.Permission{
			Id:          p.ID,
			Resource:    p.Resource,
			Action:      p.Action,
			Description: p.Description,
		}
	}
	return out
}
