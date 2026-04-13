// Package billing provides gRPC message types and server interface for billing-service.
//
// This file is a hand-written stand-in. To replace with generated code run:
//
//	protoc --go_out=. --go-grpc_out=. pkg/proto/billing/billing.proto
//
// Then delete this file and billing_grpc.go — the generated *.pb.go files take over.
package billing

// CheckSubscriptionAccessRequest is the request message for CheckSubscriptionAccess RPC.
type CheckSubscriptionAccessRequest struct {
	ClinicID string `json:"clinic_id"`
}

// CheckSubscriptionAccessResponse is the response message for CheckSubscriptionAccess RPC.
type CheckSubscriptionAccessResponse struct {
	Active bool `json:"active"`
}
