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

	"github.com/Makx3/Kite/feed-svc/internal/service"
	pb "github.com/Makx3/Kite/feed-svc/pb"
)

// FeedServer implementa pb.FeedServiceServer.
type FeedServer struct {
	pb.UnimplementedFeedServiceServer
	svc *service.FeedService
}

// NewFeedServer inyecta el servicio de negocio.
func NewFeedServer(svc *service.FeedService) *FeedServer {
	return &FeedServer{svc: svc}
}

// RefreshFeed expone service.RefreshFeed.
func (s *FeedServer) RefreshFeed(ctx context.Context, req *pb.RefreshFeedRequest) (*pb.RefreshFeedResponse, error) {
	updated, err := s.svc.RefreshFeed(ctx, req.GetAuthorId(), req.GetPostId(), req.GetCreatedAt())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.RefreshFeedResponse{Status: "ok", FeedsUpdated: int32(updated)}, nil
}

// GetUserFeed expone service.GetUserFeed.
func (s *FeedServer) GetUserFeed(ctx context.Context, req *pb.GetUserFeedRequest) (*pb.GetUserFeedResponse, error) {
	page, err := s.svc.GetUserFeed(ctx, req.GetUserId(), req.GetLimit(), req.GetOffset())
	if err != nil {
		return nil, toStatus(err)
	}
	ids := make([]string, 0, len(page.Entries))
	for _, e := range page.Entries {
		ids = append(ids, e.PostID)
	}
	return &pb.GetUserFeedResponse{
		PostIds:     ids,
		LastUpdated: page.LastUpdated,
		Total:       page.Total,
	}, nil
}

// toStatus traduce errores de dominio a códigos gRPC. Los errores
// internos se loguean con detalle y el caller recibe un mensaje
// genérico: no se filtran detalles de infraestructura.
func toStatus(err error) error {
	switch {
	case errors.Is(err, service.ErrValidation):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "operación excedió el tiempo límite")
	default:
		slog.Error("feed-svc: error interno", "error", err)
		return status.Error(codes.Internal, "error interno del servicio")
	}
}
