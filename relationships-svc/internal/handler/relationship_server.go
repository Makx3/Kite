// Package handler expone la lógica de negocio mediante el servidor
// gRPC generado. Traduce mensajes protobuf a llamadas de servicio y
// errores de dominio a códigos de estado gRPC.
package handler

import (
	"context"
	"errors"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/Makx3/Kite/relationships-svc/internal/domain"
	"github.com/Makx3/Kite/relationships-svc/internal/service"
	"github.com/Makx3/Kite/relationships-svc/internal/store"
	pb "github.com/Makx3/Kite/relationships-svc/pb"
)

// RelationshipServer implementa pb.RelationshipServiceServer.
type RelationshipServer struct {
	pb.UnimplementedRelationshipServiceServer
	svc *service.RelationshipService
}

// NewRelationshipServer inyecta el servicio de negocio.
func NewRelationshipServer(svc *service.RelationshipService) *RelationshipServer {
	return &RelationshipServer{svc: svc}
}

// FollowUser expone service.FollowUser.
func (s *RelationshipServer) FollowUser(ctx context.Context, req *pb.FollowUserRequest) (*pb.FollowUserResponse, error) {
	follow, err := s.svc.FollowUser(ctx, req.GetFollowerId(), req.GetFolloweeId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.FollowUserResponse{Follow: toProto(follow)}, nil
}

// UnfollowUser expone service.UnfollowUser.
func (s *RelationshipServer) UnfollowUser(ctx context.Context, req *pb.UnfollowUserRequest) (*pb.UnfollowUserResponse, error) {
	if err := s.svc.UnfollowUser(ctx, req.GetFollowerId(), req.GetFolloweeId()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.UnfollowUserResponse{}, nil
}

// GetFollowers expone service.GetFollowers.
func (s *RelationshipServer) GetFollowers(ctx context.Context, req *pb.GetFollowersRequest) (*pb.GetFollowersResponse, error) {
	ids, total, err := s.svc.GetFollowers(ctx, req.GetUserId(), req.GetLimit(), req.GetOffset())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.GetFollowersResponse{UserIds: ids, Total: total}, nil
}

// GetFollowing expone service.GetFollowing.
func (s *RelationshipServer) GetFollowing(ctx context.Context, req *pb.GetFollowingRequest) (*pb.GetFollowingResponse, error) {
	ids, total, err := s.svc.GetFollowing(ctx, req.GetUserId(), req.GetLimit(), req.GetOffset())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.GetFollowingResponse{UserIds: ids, Total: total}, nil
}

// toStatus traduce errores de dominio a códigos gRPC. Los errores
// internos se loguean con detalle y el caller recibe un mensaje
// genérico: no se filtran detalles de infraestructura.
func toStatus(err error) error {
	switch {
	case errors.Is(err, service.ErrValidation):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, store.ErrInvalidID):
		return status.Error(codes.InvalidArgument, store.ErrInvalidID.Error())
	case errors.Is(err, store.ErrNotFound):
		return status.Error(codes.NotFound, store.ErrNotFound.Error())
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "operación excedió el tiempo límite")
	default:
		slog.Error("relationships-svc: error interno", "error", err)
		return status.Error(codes.Internal, "error interno del servicio")
	}
}

// toProto convierte la entidad de dominio al mensaje protobuf.
func toProto(f domain.Follow) *pb.Follow {
	return &pb.Follow{
		FollowerId: f.FollowerID,
		FolloweeId: f.FolloweeID,
		CreatedAt:  timestamppb.New(f.CreatedAt),
	}
}
