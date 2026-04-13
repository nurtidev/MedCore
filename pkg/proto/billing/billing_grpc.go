package billing

import (
	"context"

	"google.golang.org/grpc"
)

// BillingServiceServer is the server interface to implement.
type BillingServiceServer interface {
	CheckSubscriptionAccess(context.Context, *CheckSubscriptionAccessRequest) (*CheckSubscriptionAccessResponse, error)
}

// RegisterBillingServiceServer registers srv with the gRPC server s.
func RegisterBillingServiceServer(s *grpc.Server, srv BillingServiceServer) {
	s.RegisterService(&billingServiceDesc, srv)
}

var billingServiceDesc = grpc.ServiceDesc{
	ServiceName: "billing.BillingService",
	HandlerType: (*BillingServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "CheckSubscriptionAccess",
			Handler:    checkSubscriptionAccessHandler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "billing/billing.proto",
}

func checkSubscriptionAccessHandler(
	srv interface{},
	ctx context.Context,
	dec func(interface{}) error,
	interceptor grpc.UnaryServerInterceptor,
) (interface{}, error) {
	in := new(CheckSubscriptionAccessRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BillingServiceServer).CheckSubscriptionAccess(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/billing.BillingService/CheckSubscriptionAccess"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BillingServiceServer).CheckSubscriptionAccess(ctx, req.(*CheckSubscriptionAccessRequest))
	}
	return interceptor(ctx, in, info, handler)
}
