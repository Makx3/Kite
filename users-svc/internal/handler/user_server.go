// Package handler expone la lógica de negocio mediante el servidor
// gRPC generado. Traduce mensajes protobuf a llamadas de servicio y
// errores de dominio a códigos de estado gRPC. No contiene lógica de
// negocio propia.
package handler

import (
	"context"
	"errors"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/Makx3/Kite/users-svc/internal/domain"
	"github.com/Makx3/Kite/users-svc/internal/service"
	"github.com/Makx3/Kite/users-svc/internal/store"
	pb "github.com/Makx3/Kite/users-svc/pb"
)

// UserServer implementa pb.UserServiceServer.
type UserServer struct {
	pb.UnimplementedUserServiceServer
	svc *service.UserService
}

// NewUserServer inyecta el servicio de negocio.
func NewUserServer(svc *service.UserService) *UserServer {
	return &UserServer{svc: svc}
}

// CreateUser expone service.CreateUser.
func (s *UserServer) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.CreateUserResponse, error) {
	user, err := s.svc.CreateUser(ctx, req.GetUsername(), req.GetEmail())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.CreateUserResponse{User: toProto(user)}, nil
}

// GetUser expone service.GetUser.
func (s *UserServer) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	user, err := s.svc.GetUser(ctx, req.GetUserId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.GetUserResponse{User: toProto(user)}, nil
}

// UpdateUser expone service.UpdateUser. Los campos no provistos en
// el protobuf llegan como nil y se ignoran (semántica de parche).
func (s *UserServer) UpdateUser(ctx context.Context, req *pb.UpdateUserRequest) (*pb.UpdateUserResponse, error) {
	user, err := s.svc.UpdateUser(ctx, req.GetUserId(), req.Username, req.Email)
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.UpdateUserResponse{User: toProto(user)}, nil
}

// toStatus traduce errores de dominio a códigos gRPC. Los errores
// internos se loguean con detalle y el caller recibe un mensaje
// genérico: no se filtran detalles de infraestructura.
func toStatus(err error) error {
	switch {
	case errors.Is(err, service.ErrValidation):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, store.ErrNotFound):
		return status.Error(codes.NotFound, store.ErrNotFound.Error())
	case errors.Is(err, store.ErrEmailTaken):
		return status.Error(codes.AlreadyExists, store.ErrEmailTaken.Error())
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "operación excedió el tiempo límite")
	default:
		slog.Error("users-svc: error interno", "error", err)
		return status.Error(codes.Internal, "error interno del servicio")
	}
}

// toProto convierte la entidad de dominio al mensaje protobuf.
func toProto(u domain.User) *pb.User {
	return &pb.User{
		UserId:    u.ID,
		Username:  u.Username,
		Email:     u.Email,
		CreatedAt: timestamppb.New(u.CreatedAt),
		UpdatedAt: timestamppb.New(u.UpdatedAt),
	}
}
