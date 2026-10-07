package main

import (
	"context"
	"log"
	"net"

	"google.golang.org/grpc"
	pb "github.com/Makx3/Kite/users-svc/pb"
)

// server implementa la interfaz generada por protoc
type server struct {
	pb.UnimplementedUserServiceServer
}

// GetUser: implementación mínima con datos hardcodeados para el laboratorio
func (s *server) GetUser(
	ctx context.Context,
	req *pb.GetUserRequest,
) (*pb.GetUserResponse, error) {
	return &pb.GetUserResponse{
		UserId:    req.UserId,
		Username:  "Maximiliano",
		Email:     "max@ufro.cl",
		CreatedAt: "2026-10-07",
	}, nil
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("no se pudo escuchar: %v", err)
	}

	srv := grpc.NewServer()
	pb.RegisterUserServiceServer(srv, &server{})

	log.Println("Servidor gRPC de Usuarios escuchando en :50051")
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("error al servir: %v", err)
	}
}
