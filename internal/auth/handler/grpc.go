package handler

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/nurtidev/medcore/internal/auth/domain"
	"github.com/nurtidev/medcore/internal/auth/service"
	authpb "github.com/nurtidev/medcore/pkg/proto/auth"
)

// GRPC implements authpb.AuthServiceServer for inter-service communication.
type GRPC struct {
	authpb.UnimplementedAuthServiceServer
	svc service.AuthService
}

// NewGRPC creates a new gRPC handler.
func NewGRPC(svc service.AuthService) *GRPC {
	return &GRPC{svc: svc}
}

// ValidateToken validates an access JWT and returns its claims.
// Called by downstream services (billing, analytics, integration) to authenticate requests.
func (g *GRPC) ValidateToken(ctx context.Context, req *authpb.ValidateTokenRequest) (*authpb.ValidateTokenResponse, error) {
	if req.AccessToken == "" {
		return nil, status.Error(codes.InvalidArgument, "access_token is required")
	}

	claims, err := g.svc.ValidateToken(ctx, req.AccessToken)
	if err != nil {
		switch err {
		case domain.ErrTokenExpired:
			return &authpb.ValidateTokenResponse{Valid: false}, nil
		case domain.ErrTokenInvalid:
			return &authpb.ValidateTokenResponse{Valid: false}, nil
		default:
			return nil, status.Errorf(codes.Internal, "validate token: %v", err)
		}
	}

	perms := make([]string, len(claims.Permissions))
	for i, p := range claims.Permissions {
		perms[i] = string(p)
	}

	return &authpb.ValidateTokenResponse{
		Valid:       true,
		UserId:      claims.UserID.String(),
		ClinicId:    claims.ClinicID.String(),
		Role:        string(claims.Role),
		Permissions: perms,
	}, nil
}

// CheckPermission checks whether a user has the given permission.
func (g *GRPC) CheckPermission(ctx context.Context, req *authpb.CheckPermissionRequest) (*authpb.CheckPermissionResponse, error) {
	if req.UserId == "" || req.Permission == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id and permission are required")
	}

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id")
	}

	allowed, err := g.svc.HasPermission(ctx, userID, domain.Permission(req.Permission))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "check permission: %v", err)
	}

	return &authpb.CheckPermissionResponse{Allowed: allowed}, nil
}
