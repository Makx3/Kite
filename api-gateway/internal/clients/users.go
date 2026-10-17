package clients

import (
	"context"
	"time"

	"google.golang.org/grpc"

	userspb "github.com/Makx3/Kite/api-gateway/pb/users"
)

// Users es el cliente gRPC del servicio de identidad.
type Users struct {
	conn    *grpc.ClientConn
	client  userspb.UserServiceClient
	timeout time.Duration
}

// NewUsers crea el cliente. No conecta de inmediato (conexión perezosa).
func NewUsers(addr string, timeout time.Duration) (*Users, error) {
	conn, err := dial(addr)
	if err != nil {
		return nil, err
	}
	return &Users{
		conn:    conn,
		client:  userspb.NewUserServiceClient(conn),
		timeout: timeout,
	}, nil
}

// Create registra un usuario nuevo.
func (c *Users) Create(ctx context.Context, username, email, displayName string) (*userspb.User, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.CreateUser(ctx, &userspb.CreateUserRequest{
		Username:    username,
		Email:       email,
		DisplayName: displayName,
	})
	if err != nil {
		return nil, err
	}
	return resp.GetUser(), nil
}

// Get obtiene un usuario por su identificador.
func (c *Users) Get(ctx context.Context, id string) (*userspb.User, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.GetUser(ctx, &userspb.GetUserRequest{UserId: id})
	if err != nil {
		return nil, err
	}
	return resp.GetUser(), nil
}

// Update aplica un parche: solo los punteros no-nulos se modifican.
func (c *Users) Update(ctx context.Context, id string, displayName, email *string) (*userspb.User, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.UpdateUser(ctx, &userspb.UpdateUserRequest{
		UserId:      id,
		DisplayName: displayName,
		Email:       email,
	})
	if err != nil {
		return nil, err
	}
	return resp.GetUser(), nil
}

// State reporta el estado de conectividad (para el health check).
func (c *Users) State() string { return stateOf(c.conn) }

// Close libera la conexión gRPC.
func (c *Users) Close() error { return c.conn.Close() }
