package main

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/dafsic/webx/internal/people/model"
	peoplev1 "github.com/dafsic/webx/proto_go/people/v1"
	"github.com/dafsic/webx/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// server implements peoplev1.PeopleServiceServer.
type server struct {
	peoplev1.UnimplementedPeopleServiceServer
	auth   *Authenticator
	repo   *Repository
	logger *slog.Logger
}

// NewServer constructs the gRPC service implementation.
func NewServer(auth *Authenticator, repo *Repository, logger *slog.Logger) peoplev1.PeopleServiceServer {
	return &server{auth: auth, repo: repo, logger: logger}
}

// GetChallenge issues a one-time nonce for the address to sign.
func (s *server) GetChallenge(ctx context.Context, req *peoplev1.GetChallengeRequest) (*peoplev1.GetChallengeResponse, error) {
	addr, err := normalizeAddress(req.GetAddress())
	if err != nil {
		return nil, err
	}
	nonce, err := s.auth.IssueChallenge(ctx, addr)
	if err != nil {
		s.logger.Error("people: issue challenge", "address", addr, "err", err)
		return nil, status.Error(codes.Internal, "failed to issue challenge")
	}
	return &peoplev1.GetChallengeResponse{Nonce: nonce}, nil
}

// Login verifies the wallet signature, auto-registers first-time wallets and
// returns a JWT session.
func (s *server) Login(ctx context.Context, req *peoplev1.LoginRequest) (*peoplev1.LoginResponse, error) {
	addr, err := normalizeAddress(req.GetAddress())
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.GetSignature()) == "" {
		return nil, status.Error(codes.InvalidArgument, "signature is required")
	}

	if err := s.auth.VerifyLogin(ctx, addr, req.GetSignature()); err != nil {
		switch {
		case errors.Is(err, ErrChallengeNotFound):
			return nil, status.Error(codes.FailedPrecondition, "challenge not found or expired; request a new one")
		case errors.Is(err, ErrBadSignature):
			return nil, status.Error(codes.Unauthenticated, "signature verification failed")
		default:
			s.logger.Error("people: verify login", "address", addr, "err", err)
			return nil, status.Error(codes.Internal, "login failed")
		}
	}

	user, err := s.repo.FindByAddress(ctx, addr)
	if err != nil {
		s.logger.Error("people: find user", "address", addr, "err", err)
		return nil, status.Error(codes.Internal, "login failed")
	}
	isNew := false
	if user == nil {
		user, err = s.repo.Create(ctx, addr)
		if err != nil {
			s.logger.Error("people: create user", "address", addr, "err", err)
			return nil, status.Error(codes.Internal, "login failed")
		}
		isNew = true
	}

	roles, err := s.repo.RolesWithPermissions(ctx, derefInt64(user.ID))
	if err != nil {
		s.logger.Error("people: load roles", "user_id", derefInt64(user.ID), "err", err)
		return nil, status.Error(codes.Internal, "login failed")
	}

	token, expiresAt, err := s.auth.IssueToken(derefInt64(user.ID), addr)
	if err != nil {
		s.logger.Error("people: issue token", "user_id", derefInt64(user.ID), "err", err)
		return nil, status.Error(codes.Internal, "login failed")
	}

	return &peoplev1.LoginResponse{
		Token:     token,
		ExpiresAt: expiresAt.Unix(),
		User:      toProtoUser(user, roles),
		IsNew:     isNew,
	}, nil
}

// Logout revokes the caller's current session token.
func (s *server) Logout(ctx context.Context, _ *peoplev1.LogoutRequest) (*peoplev1.LogoutResponse, error) {
	claims, ok := claimsFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}
	if err := s.auth.Revoke(ctx, claims); err != nil {
		s.logger.Error("people: revoke token", "jti", claims.ID, "err", err)
		return nil, status.Error(codes.Internal, "logout failed")
	}
	return &peoplev1.LogoutResponse{}, nil
}

// CheckPermission reports whether a user holds a resource+action permission.
func (s *server) CheckPermission(ctx context.Context, req *peoplev1.CheckPermissionRequest) (*peoplev1.CheckPermissionResponse, error) {
	if req.GetUserId() == 0 || req.GetResource() == "" || req.GetAction() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id, resource and action are required")
	}
	allowed, err := s.repo.HasPermission(ctx, req.GetUserId(), req.GetResource(), req.GetAction())
	if err != nil {
		s.logger.Error("people: check permission", "err", err)
		return nil, status.Error(codes.Internal, "permission check failed")
	}
	return &peoplev1.CheckPermissionResponse{Allowed: allowed}, nil
}

// normalizeAddress validates and lowercases an EVM address.
func normalizeAddress(raw string) (string, error) {
	addr := strings.ToLower(strings.TrimSpace(raw))
	if !utils.IsEthereumAddress(addr) {
		return "", status.Error(codes.InvalidArgument, "invalid EVM address")
	}
	return addr, nil
}

// toProtoUser maps a model.User and its roles to the wire User message.
func toProtoUser(u *model.User, roles []model.Role) *peoplev1.User {
	pbRoles := make([]*peoplev1.Role, 0, len(roles))
	for i := range roles {
		pbRoles = append(pbRoles, toProtoRole(&roles[i]))
	}
	out := &peoplev1.User{
		Id:        derefInt64(u.ID),
		Address:   derefString(u.Address),
		Nickname:  derefString(u.Nickname),
		AvatarUrl: derefString(u.AvatarURL),
		Email:     derefString(u.Email),
		Roles:     pbRoles,
	}
	if u.CreatedAt != nil {
		out.CreatedAt = u.CreatedAt.Unix()
	}
	if u.UpdatedAt != nil {
		out.UpdatedAt = u.UpdatedAt.Unix()
	}
	return out
}

func toProtoRole(r *model.Role) *peoplev1.Role {
	pbPerms := make([]*peoplev1.Permission, 0, len(r.Permissions))
	for i := range r.Permissions {
		p := &r.Permissions[i]
		pbPerms = append(pbPerms, &peoplev1.Permission{
			Id:          derefInt64(p.ID),
			Resource:    derefString(p.Resource),
			Action:      derefString(p.Action),
			Description: derefString(p.Description),
		})
	}
	return &peoplev1.Role{
		Id:          derefInt64(r.ID),
		Name:        derefString(r.Name),
		Description: derefString(r.Description),
		Permissions: pbPerms,
	}
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
