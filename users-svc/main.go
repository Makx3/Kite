package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/Makx3/Kite/users-svc/pb"
)

// user guarda los datos de un usuario creado durante la demo
type user struct {
	username  string
	email     string
	createdAt string
}

// server implementa la interfaz generada por protoc
type server struct {
	pb.UnimplementedUserServiceServer

	mu     sync.RWMutex
	users  map[string]user
	nextID int
}

// CreateUser: mock con almacenamiento en memoria (sin base de datos, como pide el lab)
func (s *server) CreateUser(
	ctx context.Context,
	req *pb.CreateUserRequest,
) (*pb.CreateUserResponse, error) {
	if req.Username == "" || req.Email == "" {
		return nil, status.Error(codes.InvalidArgument, "username y email son obligatorios")
	}

	s.mu.Lock()
	s.nextID++
	id := fmt.Sprintf("usr-%03d", s.nextID)
	s.users[id] = user{
		username:  req.Username,
		email:     req.Email,
		createdAt: time.Now().Format(time.RFC3339),
	}
	s.mu.Unlock()

	log.Printf("usuario creado: %s (%s)", id, req.Username)

	return &pb.CreateUserResponse{
		UserId: id,
		Status: "CREATED",
	}, nil
}

// GetUser: busca en el mock en memoria y, si no existe, devuelve datos de ejemplo
func (s *server) GetUser(
	ctx context.Context,
	req *pb.GetUserRequest,
) (*pb.GetUserResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id es obligatorio")
	}

	s.mu.RLock()
	u, ok := s.users[req.UserId]
	s.mu.RUnlock()

	if !ok {
		return &pb.GetUserResponse{
			UserId:    req.UserId,
			Username:  "Maximiliano",
			Email:     "max@ufro.cl",
			CreatedAt: "2026-10-07",
		}, nil
	}

	return &pb.GetUserResponse{
		UserId:    req.UserId,
		Username:  u.username,
		Email:     u.email,
		CreatedAt: u.createdAt,
	}, nil
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("no se pudo escuchar: %v", err)
	}

	srv := grpc.NewServer()
	pb.RegisterUserServiceServer(srv, &server{
		users: make(map[string]user),
	})

	log.Println("Servidor gRPC de Usuarios escuchando en :50051")
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("error al servir: %v", err)
	}
}
