package handler

import (
	"context"

	"github.com/google/uuid"
	"github.com/nurtidev/medcore/internal/billing/service"
	billingpb "github.com/nurtidev/medcore/pkg/proto/billing"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GRPC implements billingpb.BillingServiceServer for inter-service communication.
// Other services (gateway, appointment-service) call CheckSubscriptionAccess to gate
// feature access based on a clinic's active subscription.
type GRPC struct {
	svc service.BillingService
}

// NewGRPC creates a new billing gRPC handler.
func NewGRPC(svc service.BillingService) *GRPC {
	return &GRPC{svc: svc}
}

// CheckSubscriptionAccess reports whether a clinic has an active subscription.
func (g *GRPC) CheckSubscriptionAccess(
	ctx context.Context,
	req *billingpb.CheckSubscriptionAccessRequest,
) (*billingpb.CheckSubscriptionAccessResponse, error) {
	if req.ClinicID == "" {
		return nil, status.Error(codes.InvalidArgument, "clinic_id is required")
	}
	clinicID, err := uuid.Parse(req.ClinicID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid clinic_id: %v", err)
	}

	active, err := g.svc.CheckSubscriptionAccess(ctx, clinicID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "check subscription access: %v", err)
	}
	return &billingpb.CheckSubscriptionAccessResponse{Active: active}, nil
}

// Register registers the billing gRPC service with the given server.
func (g *GRPC) Register(srv *grpc.Server) {
	billingpb.RegisterBillingServiceServer(srv, g)
}
