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

	"github.com/Makx3/Kite/likes-svc/internal/service"
	"github.com/Makx3/Kite/likes-svc/internal/store"
	pb "github.com/Makx3/Kite/likes-svc/pb"
)

// LikeServer implementa pb.LikeServiceServer.
type LikeServer struct {
	pb.UnimplementedLikeServiceServer
	svc *service.LikeService
}

// NewLikeServer inyecta el servicio de negocio.
func NewLikeServer(svc *service.LikeService) *LikeServer {
	return &LikeServer{svc: svc}
}

// LikePost expone service.LikePost.
func (s *LikeServer) LikePost(ctx context.Context, req *pb.LikePostRequest) (*pb.LikePostResponse, error) {
	like, created, err := s.svc.LikePost(ctx, req.GetPostId(), req.GetUserId(), req.GetPostAuthorId())
	if err != nil {
		return nil, toStatus(err)
	}
	statusMsg := "ok"
	if !created {
		statusMsg = "already_liked"
	}
	return &pb.LikePostResponse{LikeId: like.ID, Status: statusMsg}, nil
}

// UnlikePost expone service.UnlikePost.
func (s *LikeServer) UnlikePost(ctx context.Context, req *pb.UnlikePostRequest) (*pb.UnlikePostResponse, error) {
	if err := s.svc.UnlikePost(ctx, req.GetPostId(), req.GetUserId()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.UnlikePostResponse{Status: "ok"}, nil
}

// GetPostLikes expone service.GetPostLikes.
func (s *LikeServer) GetPostLikes(ctx context.Context, req *pb.GetPostLikesRequest) (*pb.GetPostLikesResponse, error) {
	ids, total, err := s.svc.GetPostLikes(ctx, req.GetPostId(), req.GetLimit(), req.GetOffset())
	if err != nil {
		return nil, toStatus(err)
	}
	if ids == nil {
		ids = []string{}
	}
	return &pb.GetPostLikesResponse{UserIds: ids, Total: total}, nil
}

// HasLiked expone service.HasLiked.
func (s *LikeServer) HasLiked(ctx context.Context, req *pb.HasLikedRequest) (*pb.HasLikedResponse, error) {
	liked, err := s.svc.HasLiked(ctx, req.GetPostId(), req.GetUserId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.HasLikedResponse{Liked: liked}, nil
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
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "operación excedió el tiempo límite")
	default:
		slog.Error("likes-svc: error interno", "error", err)
		return status.Error(codes.Internal, "error interno del servicio")
	}
}
