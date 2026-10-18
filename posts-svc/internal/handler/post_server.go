// Package handler expone la lógica de negocio mediante el servidor gRPC.
package handler

import (
	"context"
	"errors"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/Makx3/Kite/posts-svc/internal/domain"
	"github.com/Makx3/Kite/posts-svc/internal/service"
	"github.com/Makx3/Kite/posts-svc/internal/store"
	pb "github.com/Makx3/Kite/posts-svc/pb"
)

type PostServer struct {
	pb.UnimplementedPostServiceServer
	svc *service.UserService
}

func NewPostServer(svc *service.UserService) *PostServer {
	return &PostServer{svc: svc}
}

func (s *PostServer) CreatePost(ctx context.Context, req *pb.CreatePostRequest) (*pb.CreatePostResponse, error) {
	post, err := s.svc.CreatePost(ctx, req.GetUserId(), req.GetContent())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.CreatePostResponse{Post: toProto(post)}, nil
}

func (s *PostServer) GetPost(ctx context.Context, req *pb.GetPostRequest) (*pb.GetPostResponse, error) {
	post, err := s.svc.GetPost(ctx, req.GetPostId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &pb.GetPostResponse{Post: toProto(post)}, nil
}

func (s *PostServer) GetPostsByIds(ctx context.Context, req *pb.GetPostsByIdsRequest) (*pb.GetPostsByIdsResponse, error) {
	posts, missing, err := s.svc.GetPostsByIDs(ctx, req.GetPostIds())
	if err != nil {
		return nil, toStatus(err)
	}
	res := make([]*pb.Post, 0, len(posts))
	for i := range posts {
		res = append(res, toProto(posts[i]))
	}
	return &pb.GetPostsByIdsResponse{Posts: res, Missing: missing}, nil
}

func (s *PostServer) GetPostsByUser(ctx context.Context, req *pb.GetPostsByUserRequest) (*pb.GetPostsByUserResponse, error) {
	posts, total, err := s.svc.GetPostsByUser(ctx, req.GetUserId(), req.GetLimit(), req.GetOffset())
	if err != nil {
		return nil, toStatus(err)
	}
	res := make([]*pb.Post, 0, len(posts))
	for i := range posts {
		res = append(res, toProto(posts[i]))
	}
	return &pb.GetPostsByUserResponse{Posts: res, Total: total}, nil
}

func (s *PostServer) DeletePost(ctx context.Context, req *pb.DeletePostRequest) (*pb.DeletePostResponse, error) {
	if err := s.svc.DeletePost(ctx, req.GetPostId(), req.GetUserId()); err != nil {
		return nil, toStatus(err)
	}
	return &pb.DeletePostResponse{Status: "ok"}, nil
}

func toStatus(err error) error {
	switch {
	case errors.Is(err, service.ErrValidation):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, store.ErrNotFound):
		return status.Error(codes.NotFound, store.ErrNotFound.Error())
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "operación excedió el tiempo límite")
	default:
		slog.Error("posts-svc: error interno", "error", err)
		return status.Error(codes.Internal, "error interno del servicio")
	}
}

func toProto(p domain.Post) *pb.Post {
	return &pb.Post{
		PostId:    p.ID,
		UserId:    p.UserID,
		Content:   p.Content,
		CreatedAt: timestamppb.New(p.CreatedAt),
	}
}
