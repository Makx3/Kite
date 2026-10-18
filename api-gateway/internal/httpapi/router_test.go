package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	likespb "github.com/Makx3/Kite/api-gateway/pb/likes"
	notificationspb "github.com/Makx3/Kite/api-gateway/pb/notifications"
	postspb "github.com/Makx3/Kite/api-gateway/pb/posts"
	relationshipspb "github.com/Makx3/Kite/api-gateway/pb/relationships"
	userspb "github.com/Makx3/Kite/api-gateway/pb/users"
)

// ─────────────────────────────────────────────────────────
// Dobles en memoria de los tres puertos. Gracias a las interfaces
// (ports.go) el router se prueba sin red ni contenedores.
// ─────────────────────────────────────────────────────────

type fakeUsers struct {
	createFn func(username, email, displayName string) (*userspb.User, error)
	getFn    func(id string) (*userspb.User, error)
	updateFn func(id string, displayName, email *string) (*userspb.User, error)

	mu       sync.Mutex
	getCalls []string
}

func (f *fakeUsers) Create(_ context.Context, username, email, displayName string) (*userspb.User, error) {
	return f.createFn(username, email, displayName)
}

func (f *fakeUsers) Get(_ context.Context, id string) (*userspb.User, error) {
	f.mu.Lock()
	f.getCalls = append(f.getCalls, id)
	f.mu.Unlock()
	return f.getFn(id)
}

func (f *fakeUsers) Update(_ context.Context, id string, displayName, email *string) (*userspb.User, error) {
	return f.updateFn(id, displayName, email)
}

func (f *fakeUsers) State() string { return "READY" }

type fakeRelationships struct {
	followFn      func(followerID, followeeID string) (*relationshipspb.Follow, error)
	unfollowFn    func(followerID, followeeID string) error
	listFn        func(userID string, limit, offset int32) ([]string, int32, error)
	gotPagination struct{ limit, offset int32 }
}

func (f *fakeRelationships) Follow(_ context.Context, followerID, followeeID string) (*relationshipspb.Follow, error) {
	return f.followFn(followerID, followeeID)
}

func (f *fakeRelationships) Unfollow(_ context.Context, followerID, followeeID string) error {
	return f.unfollowFn(followerID, followeeID)
}

func (f *fakeRelationships) Followers(_ context.Context, userID string, limit, offset int32) ([]string, int32, error) {
	f.gotPagination.limit, f.gotPagination.offset = limit, offset
	return f.listFn(userID, limit, offset)
}

func (f *fakeRelationships) Following(_ context.Context, userID string, limit, offset int32) ([]string, int32, error) {
	f.gotPagination.limit, f.gotPagination.offset = limit, offset
	return f.listFn(userID, limit, offset)
}

func (f *fakeRelationships) State() string { return "READY" }

type fakeNotifications struct {
	unreadFn func(userID string, limit, offset int32) ([]*notificationspb.Notification, int32, error)
	readFn   func(userID, notificationID string) (*notificationspb.Notification, error)
}

func (f *fakeNotifications) Unread(_ context.Context, userID string, limit, offset int32) ([]*notificationspb.Notification, int32, error) {
	return f.unreadFn(userID, limit, offset)
}

func (f *fakeNotifications) MarkRead(_ context.Context, userID, notificationID string) (*notificationspb.Notification, error) {
	return f.readFn(userID, notificationID)
}

func (f *fakeNotifications) State() string { return "READY" }

type fakePosts struct {
	createFn   func(userID, content string) (*postspb.Post, error)
	getFn      func(postID string) (*postspb.Post, error)
	getByIDsFn func(ids []string) ([]*postspb.Post, error)
}

func (f *fakePosts) Create(_ context.Context, userID, content string) (*postspb.Post, error) {
	return f.createFn(userID, content)
}

func (f *fakePosts) Get(_ context.Context, postID string) (*postspb.Post, error) {
	return f.getFn(postID)
}

func (f *fakePosts) GetByIDs(_ context.Context, ids []string) ([]*postspb.Post, error) {
	return f.getByIDsFn(ids)
}

func (f *fakePosts) State() string { return "READY" }

type fakeFeed struct {
	userFeedFn    func(userID string, limit, offset int32) ([]string, int32, string, error)
	gotPagination struct{ limit, offset int32 }
}

func (f *fakeFeed) UserFeed(_ context.Context, userID string, limit, offset int32) ([]string, int32, string, error) {
	f.gotPagination.limit, f.gotPagination.offset = limit, offset
	return f.userFeedFn(userID, limit, offset)
}

func (f *fakeFeed) State() string { return "READY" }

type fakeLikes struct {
	likeFn    func(postID, userID, postAuthorID string) (likeID, status string, err error)
	unlikeFn  func(postID, userID string) error
	gotAuthor string
}

func (f *fakeLikes) Like(_ context.Context, postID, userID, postAuthorID string) (*likespb.LikePostResponse, error) {
	f.gotAuthor = postAuthorID
	likeID, status, err := f.likeFn(postID, userID, postAuthorID)
	if err != nil {
		return nil, err
	}
	return &likespb.LikePostResponse{LikeId: likeID, Status: status}, nil
}

func (f *fakeLikes) Unlike(_ context.Context, postID, userID string) error {
	return f.unlikeFn(postID, userID)
}

func (f *fakeLikes) State() string { return "READY" }

// ─────────────────────────────────────────────────────────
// Utilidades
// ─────────────────────────────────────────────────────────

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// testBackends reúne los dobles de los seis backends. Cada campo nil
// se reemplaza por un doble permisivo por defecto.
type testBackends struct {
	users *fakeUsers
	rel   *fakeRelationships
	notif *fakeNotifications
	posts *fakePosts
	feed  *fakeFeed
	likes *fakeLikes
}

// newTestRouter construye el router con dobles permisivos por defecto;
// cada prueba sobreescribe solo lo que le interesa.
func newTestRouter(t *testing.T, u *fakeUsers, rel *fakeRelationships, n *fakeNotifications) http.Handler {
	t.Helper()
	return newTestRouterWith(t, testBackends{users: u, rel: rel, notif: n})
}

func newTestRouterWith(t *testing.T, b testBackends) http.Handler {
	t.Helper()
	u := b.users
	if u == nil {
		u = &fakeUsers{
			createFn: func(username, email, displayName string) (*userspb.User, error) {
				return &userspb.User{UserId: "u-1", Username: username, Email: email, DisplayName: displayName}, nil
			},
			getFn: func(id string) (*userspb.User, error) {
				return &userspb.User{UserId: id, Username: "diego", Email: "d@ufro.cl", DisplayName: "Diego"}, nil
			},
			updateFn: func(id string, displayName, email *string) (*userspb.User, error) {
				return &userspb.User{UserId: id, Username: "diego"}, nil
			},
		}
	}
	rel := b.rel
	if rel == nil {
		rel = &fakeRelationships{
			followFn: func(followerID, followeeID string) (*relationshipspb.Follow, error) {
				return &relationshipspb.Follow{
					FollowerId: followerID,
					FolloweeId: followeeID,
					CreatedAt:  timestamppb.New(time.Unix(0, 0).UTC()),
				}, nil
			},
			unfollowFn: func(string, string) error { return nil },
			listFn:     func(string, int32, int32) ([]string, int32, error) { return []string{"a"}, 1, nil },
		}
	}
	n := b.notif
	if n == nil {
		n = &fakeNotifications{
			unreadFn: func(string, int32, int32) ([]*notificationspb.Notification, int32, error) {
				return nil, 0, nil
			},
			readFn: func(userID, notificationID string) (*notificationspb.Notification, error) {
				return &notificationspb.Notification{NotificationId: notificationID, UserId: userID}, nil
			},
		}
	}
	p := b.posts
	if p == nil {
		p = &fakePosts{
			createFn: func(userID, content string) (*postspb.Post, error) {
				return &postspb.Post{
					PostId:    "p-1",
					UserId:    userID,
					Content:   content,
					CreatedAt: timestamppb.New(time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)),
				}, nil
			},
			getFn: func(postID string) (*postspb.Post, error) {
				return &postspb.Post{
					PostId:    postID,
					UserId:    "author-1",
					Content:   "hola",
					CreatedAt: timestamppb.New(time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)),
				}, nil
			},
			getByIDsFn: func(ids []string) ([]*postspb.Post, error) {
				out := make([]*postspb.Post, 0, len(ids))
				for _, id := range ids {
					out = append(out, &postspb.Post{PostId: id, UserId: "author-1", Content: "hola " + id})
				}
				return out, nil
			},
		}
	}
	f := b.feed
	if f == nil {
		f = &fakeFeed{
			userFeedFn: func(string, int32, int32) ([]string, int32, string, error) {
				return []string{"p-1"}, 1, "2026-10-18T12:00:00Z", nil
			},
		}
	}
	l := b.likes
	if l == nil {
		l = &fakeLikes{
			likeFn: func(postID, userID, postAuthorID string) (string, string, error) {
				return "like-1", "ok", nil
			},
			unlikeFn: func(string, string) error { return nil },
		}
	}
	return NewRouter(Deps{
		Users:          u,
		Relationships:  rel,
		Notifications:  n,
		Posts:          p,
		Feed:           f,
		Likes:          l,
		HandlerTimeout: 2 * time.Second,
	}, discardLogger())
}

// do ejecuta una petición contra el router y devuelve la respuesta.
func do(t *testing.T, h http.Handler, method, target, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, rdr)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("respuesta no es JSON válido (%q): %v", rec.Body.String(), err)
	}
	return out
}

// ─────────────────────────────────────────────────────────
// Usuarios
// ─────────────────────────────────────────────────────────

func TestCreateUserResponde201(t *testing.T) {
	var got struct{ username, email, displayName string }
	u := &fakeUsers{
		createFn: func(username, email, displayName string) (*userspb.User, error) {
			got.username, got.email, got.displayName = username, email, displayName
			return &userspb.User{
				UserId: "u-7", Username: username, Email: email, DisplayName: displayName,
				CreatedAt: timestamppb.New(time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)),
			}, nil
		},
	}
	rec := do(t, newTestRouter(t, u, nil, nil), http.MethodPost, "/users",
		`{"username":"diego","email":"d@ufro.cl","display_name":"Diego"}`, nil)

	if rec.Code != http.StatusCreated {
		t.Fatalf("esperaba 201, obtuve %d (%s)", rec.Code, rec.Body.String())
	}
	if got.username != "diego" || got.email != "d@ufro.cl" || got.displayName != "Diego" {
		t.Errorf("el gateway no reenvió los campos: %+v", got)
	}
	body := decodeBody(t, rec)
	if body["user_id"] != "u-7" || body["display_name"] != "Diego" {
		t.Errorf("cuerpo inesperado: %v", body)
	}
}

func TestCreateUserCuerpoInvalido(t *testing.T) {
	rec := do(t, newTestRouter(t, nil, nil, nil), http.MethodPost, "/users", `{no json`, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperaba 400, obtuve %d", rec.Code)
	}
}

func TestGetUserNoEncontrado(t *testing.T) {
	u := &fakeUsers{getFn: func(string) (*userspb.User, error) {
		return nil, status.Error(codes.NotFound, "usuario no encontrado")
	}}
	rec := do(t, newTestRouter(t, u, nil, nil), http.MethodGet, "/users/u-9", "", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("esperaba 404, obtuve %d", rec.Code)
	}
	if body := decodeBody(t, rec); body["message"] != "usuario no encontrado" {
		t.Errorf("mensaje no propagado: %v", body)
	}
}

func TestUpdateUserEsParche(t *testing.T) {
	var gotName, gotEmail *string
	u := &fakeUsers{updateFn: func(id string, displayName, email *string) (*userspb.User, error) {
		gotName, gotEmail = displayName, email
		return &userspb.User{UserId: id, DisplayName: "Nuevo"}, nil
	}}
	rec := do(t, newTestRouter(t, u, nil, nil), http.MethodPatch, "/users/u-1",
		`{"display_name":"Nuevo"}`, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba 200, obtuve %d (%s)", rec.Code, rec.Body.String())
	}
	if gotName == nil || *gotName != "Nuevo" {
		t.Errorf("display_name no llegó: %v", gotName)
	}
	if gotEmail != nil {
		t.Errorf("email debía llegar nil (ausente en el cuerpo), llegó %v", *gotEmail)
	}
}

func TestEmailYaRegistradoResponde409(t *testing.T) {
	u := &fakeUsers{createFn: func(string, string, string) (*userspb.User, error) {
		return nil, status.Error(codes.AlreadyExists, "email ya registrado")
	}}
	rec := do(t, newTestRouter(t, u, nil, nil), http.MethodPost, "/users", `{"username":"x"}`, nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("esperaba 409, obtuve %d", rec.Code)
	}
}

// ─────────────────────────────────────────────────────────
// Grafo de seguimiento
// ─────────────────────────────────────────────────────────

func TestFollowUsaEncabezadoComoSeguidor(t *testing.T) {
	var gotFollower, gotFollowee string
	rel := &fakeRelationships{followFn: func(followerID, followeeID string) (*relationshipspb.Follow, error) {
		gotFollower, gotFollowee = followerID, followeeID
		return &relationshipspb.Follow{FollowerId: followerID, FolloweeId: followeeID}, nil
	}}
	rec := do(t, newTestRouter(t, nil, rel, nil), http.MethodPost, "/users/u-2/follow", "",
		map[string]string{"X-User-Id": "u-1"})

	if rec.Code != http.StatusCreated {
		t.Fatalf("esperaba 201, obtuve %d (%s)", rec.Code, rec.Body.String())
	}
	if gotFollower != "u-1" || gotFollowee != "u-2" {
		t.Errorf("seguidor/seguido mal resueltos: %q -> %q", gotFollower, gotFollowee)
	}
}

func TestFollowSinEncabezadoResponde400(t *testing.T) {
	rec := do(t, newTestRouter(t, nil, nil, nil), http.MethodPost, "/users/u-2/follow", "", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperaba 400, obtuve %d", rec.Code)
	}
}

func TestUnfollowResponde204(t *testing.T) {
	rec := do(t, newTestRouter(t, nil, nil, nil), http.MethodDelete, "/users/u-2/follow", "",
		map[string]string{"X-User-Id": "u-1"})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("esperaba 204, obtuve %d", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("204 no debe llevar cuerpo, lleva %q", rec.Body.String())
	}
}

func TestFollowersReenviaPaginacion(t *testing.T) {
	rel := &fakeRelationships{listFn: func(string, int32, int32) ([]string, int32, error) {
		return []string{"a", "b"}, 7, nil
	}}
	rec := do(t, newTestRouter(t, nil, rel, nil), http.MethodGet,
		"/users/u-1/followers?limit=2&offset=4", "", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba 200, obtuve %d", rec.Code)
	}
	if rel.gotPagination.limit != 2 || rel.gotPagination.offset != 4 {
		t.Errorf("paginación mal reenviada: %+v", rel.gotPagination)
	}
	body := decodeBody(t, rec)
	if body["total"].(float64) != 7 {
		t.Errorf("total inesperado: %v", body["total"])
	}
}

func TestFollowersPaginacionInvalida(t *testing.T) {
	rec := do(t, newTestRouter(t, nil, nil, nil), http.MethodGet,
		"/users/u-1/followers?limit=abc", "", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperaba 400, obtuve %d", rec.Code)
	}
}

func TestPaginaVaciaNoEsNull(t *testing.T) {
	rel := &fakeRelationships{listFn: func(string, int32, int32) ([]string, int32, error) {
		return nil, 0, nil
	}}
	rec := do(t, newTestRouter(t, nil, rel, nil), http.MethodGet, "/users/u-1/followers", "", nil)
	if !strings.Contains(rec.Body.String(), `"user_ids":[]`) {
		t.Errorf("esperaba lista vacía, obtuve %s", rec.Body.String())
	}
}

// ─────────────────────────────────────────────────────────
// Notificaciones (agregación)
// ─────────────────────────────────────────────────────────

func TestListNotificationsHidrataEmisor(t *testing.T) {
	n := &fakeNotifications{unreadFn: func(userID string, limit, offset int32) ([]*notificationspb.Notification, int32, error) {
		return []*notificationspb.Notification{
			{NotificationId: "n-1", Type: notificationspb.NotificationType_NOTIFICATION_TYPE_NEW_FOLLOWER, UserId: userID, SourceUserId: "u-1"},
			{NotificationId: "n-2", Type: notificationspb.NotificationType_NOTIFICATION_TYPE_NEW_FOLLOWER, UserId: userID, SourceUserId: "u-1"},
			{NotificationId: "n-3", Type: notificationspb.NotificationType_NOTIFICATION_TYPE_LIKE, UserId: userID, SourceUserId: "u-2", PostId: strPtr("p-9")},
		}, 3, nil
	}}
	u := &fakeUsers{getFn: func(id string) (*userspb.User, error) {
		return &userspb.User{UserId: id, Username: "user-" + id, DisplayName: "Nombre " + id}, nil
	}}

	rec := do(t, newTestRouter(t, u, nil, n), http.MethodGet, "/notifications", "",
		map[string]string{"X-User-Id": "u-9"})

	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba 200, obtuve %d (%s)", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body["total"].(float64) != 3 {
		t.Errorf("total inesperado: %v", body["total"])
	}
	list := body["notifications"].([]any)
	first := list[0].(map[string]any)
	src, ok := first["source_user"].(map[string]any)
	if !ok {
		t.Fatalf("source_user no fue hidratado: %v", first)
	}
	if src["display_name"] != "Nombre u-1" {
		t.Errorf("emisor mal hidratado: %v", src)
	}
	if first["type"] != "NEW_FOLLOWER" {
		t.Errorf("tipo mal traducido: %v", first["type"])
	}
	third := list[2].(map[string]any)
	if third["type"] != "LIKE" || third["post_id"] != "p-9" {
		t.Errorf("alerta LIKE mal representada: %v", third)
	}

	// Deduplicación: 3 alertas con 2 emisores distintos => 2 consultas.
	if len(u.getCalls) != 2 {
		t.Errorf("esperaba 2 llamadas a Users (deduplicadas), hubo %d: %v", len(u.getCalls), u.getCalls)
	}
}

func TestListNotificationsDegradaSiUsersFalla(t *testing.T) {
	n := &fakeNotifications{unreadFn: func(userID string, _, _ int32) ([]*notificationspb.Notification, int32, error) {
		return []*notificationspb.Notification{
			{NotificationId: "n-1", UserId: userID, SourceUserId: "u-1"},
		}, 1, nil
	}}
	// Users caído: la bandeja debe entregarse igual, sin el emisor.
	u := &fakeUsers{getFn: func(string) (*userspb.User, error) {
		return nil, status.Error(codes.Unavailable, "caído")
	}}

	rec := do(t, newTestRouter(t, u, nil, n), http.MethodGet, "/notifications", "",
		map[string]string{"X-User-Id": "u-9"})

	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba 200 pese al fallo de Users, obtuve %d", rec.Code)
	}
	body := decodeBody(t, rec)
	list := body["notifications"].([]any)
	if _, present := list[0].(map[string]any)["source_user"]; present {
		t.Error("no debía incluirse source_user cuando Users falla")
	}
	if list[0].(map[string]any)["notification_id"] != "n-1" {
		t.Errorf("la alerta debía entregarse igual: %v", list[0])
	}
}

func TestListNotificationsSinUsuarioResponde400(t *testing.T) {
	rec := do(t, newTestRouter(t, nil, nil, nil), http.MethodGet, "/notifications", "", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperaba 400, obtuve %d", rec.Code)
	}
}

func TestMarkReadAceptaUsuarioEnElCuerpo(t *testing.T) {
	var gotUser, gotNotif string
	n := &fakeNotifications{readFn: func(userID, notificationID string) (*notificationspb.Notification, error) {
		gotUser, gotNotif = userID, notificationID
		return &notificationspb.Notification{NotificationId: notificationID, UserId: userID, IsRead: true}, nil
	}}

	rec := do(t, newTestRouter(t, nil, nil, n), http.MethodPost, "/notifications/n-5/read",
		`{"user_id":"u-3"}`, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba 200, obtuve %d (%s)", rec.Code, rec.Body.String())
	}
	if gotUser != "u-3" || gotNotif != "n-5" {
		t.Errorf("argumentos mal resueltos: user=%q notif=%q", gotUser, gotNotif)
	}
	if body := decodeBody(t, rec); body["is_read"] != true {
		t.Errorf("is_read no reflejado: %v", body)
	}
}

// ─────────────────────────────────────────────────────────
// Resiliencia y enrutado
// ─────────────────────────────────────────────────────────

func TestServicioCaidoResponde503(t *testing.T) {
	u := &fakeUsers{getFn: func(string) (*userspb.User, error) {
		return nil, status.Error(codes.Unavailable, "connection refused")
	}}
	rec := do(t, newTestRouter(t, u, nil, nil), http.MethodGet, "/users/u-1", "", nil)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("esperaba 503, obtuve %d", rec.Code)
	}
	body := decodeBody(t, rec)
	if body["error"] != "service_unavailable" {
		t.Errorf("código de error inesperado: %v", body)
	}
}

func TestErrorInternoNoFiltraDetalles(t *testing.T) {
	u := &fakeUsers{getFn: func(string) (*userspb.User, error) {
		return nil, errors.New("password super secreta en el DSN")
	}}
	rec := do(t, newTestRouter(t, u, nil, nil), http.MethodGet, "/users/u-1", "", nil)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("esperaba 502, obtuve %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "secreta") {
		t.Errorf("el detalle interno se filtró al cliente: %s", rec.Body.String())
	}
}

func TestRutaInexistenteResponde404(t *testing.T) {
	rec := do(t, newTestRouter(t, nil, nil, nil), http.MethodGet, "/no-existe", "", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("esperaba 404, obtuve %d", rec.Code)
	}
	// El 404 debe usar el mismo cuerpo que el resto de la API, no el
	// "404 page not found" en texto plano del enrutador de Go.
	body := decodeBody(t, rec)
	if body["error"] != "not_found" {
		t.Fatalf("cuerpo inesperado para una ruta inexistente: %v", body)
	}
}

func TestMetodoNoPermitidoResponde405(t *testing.T) {
	rec := do(t, newTestRouter(t, nil, nil, nil), http.MethodDelete, "/users", "", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("esperaba 405, obtuve %d", rec.Code)
	}
}

// La raíz del gateway es una API, no un sitio: GET / devuelve un índice con
// las rutas del contrato para que abrir la URL base en el navegador sea útil.
func TestRaizDevuelveIndiceDeLaAPI(t *testing.T) {
	rec := do(t, newTestRouter(t, nil, nil, nil), http.MethodGet, "/", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba 200, obtuve %d", rec.Code)
	}
	body := decodeBody(t, rec)
	if body["service"] == nil || body["endpoints"] == nil {
		t.Fatalf("el índice no describe la API: %v", body)
	}
	rutas, ok := body["endpoints"].([]any)
	if !ok || len(rutas) == 0 {
		t.Fatalf("endpoints mal formados: %v", body["endpoints"])
	}
	// La raíz debe anunciarse con el camino que ve el cliente, no con el
	// patrón interno de ServeMux.
	var vistos []string
	for _, r := range rutas {
		fila, _ := r.(map[string]any)
		if fila["path"] == "/{$}" {
			t.Errorf("el índice filtra el patrón interno del enrutador: %v", fila)
		}
		vistos = append(vistos, fila["method"].(string)+" "+fila["path"].(string))
	}
	for _, esperada := range []string{
		"POST /users", "GET /users/{user_id}", "GET /notifications",
		"POST /posts", "POST /posts/{post_id}/like", "GET /feed", "GET /health",
	} {
		if !slices.Contains(vistos, esperada) {
			t.Errorf("falta la ruta %q en el índice: %v", esperada, vistos)
		}
	}
}

// El patrón "GET /{$}" solo coincide con la raíz exacta: un comodín "/" se
// tragaría los caminos con método incorrecto y rompería estos 405.
func TestRaizConOtroMetodoResponde405(t *testing.T) {
	rec := do(t, newTestRouter(t, nil, nil, nil), http.MethodDelete, "/", "", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("esperaba 405, obtuve %d", rec.Code)
	}
}

// La barra final es la variante que más se teclea en el navegador: no debe
// producir 404, sino llegar al mismo manejador.
func TestBarraFinalEsTolerada(t *testing.T) {
	// POST /users/{id}/follow exige X-User-Id; sin él el manejador responde
	// 400. Si llegó al manejador, la barra final se toleró; si no, sería 404.
	rec := do(t, newTestRouter(t, nil, nil, nil), http.MethodPost, "/users/u-1/follow/", "", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperaba 400 (la petición llegó al manejador), obtuve %d", rec.Code)
	}
	if body := decodeBody(t, rec); body["error"] != "missing_user" {
		t.Fatalf("cuerpo inesperado: %v", body)
	}

	// Una lectura, con el backend de prueba: debe devolver el usuario.
	rec = do(t, newTestRouter(t, nil, nil, nil), http.MethodGet, "/users/u-1/", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba 200 para GET /users/u-1/, obtuve %d", rec.Code)
	}
	if body := decodeBody(t, rec); body["username"] != "diego" {
		t.Fatalf("usuario inesperado: %v", body)
	}
}

// Tolerar la barra final no debe inventar rutas: un camino que no existe sigue
// dando 404, con o sin barra.
func TestBarraFinalNoInventaRutas(t *testing.T) {
	for _, camino := range []string{"/no-existe", "/no-existe/", "/users/u-1/nada/"} {
		rec := do(t, newTestRouter(t, nil, nil, nil), http.MethodGet, camino, "", nil)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: esperaba 404, obtuve %d", camino, rec.Code)
			continue
		}
		if body := decodeBody(t, rec); body["error"] != "not_found" {
			t.Errorf("%s: cuerpo inesperado: %v", camino, body)
		}
	}
}

func TestHealthReportaBackends(t *testing.T) {
	rec := do(t, newTestRouter(t, nil, nil, nil), http.MethodGet, "/health", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba 200, obtuve %d", rec.Code)
	}
	body := decodeBody(t, rec)
	if body["status"] != "ok" {
		t.Errorf("estado inesperado: %v", body["status"])
	}
	backends, ok := body["backends"].(map[string]any)
	if !ok || backends["users"] != "READY" {
		t.Errorf("backends mal reportados: %v", body["backends"])
	}
	// Los seis servicios de negocio deben aparecer en la vitalidad.
	for _, nombre := range []string{"users", "relationships", "notifications", "posts", "feed", "likes"} {
		if backends[nombre] != "READY" {
			t.Errorf("falta el backend %q en /health: %v", nombre, body["backends"])
		}
	}
}

// ─────────────────────────────────────────────────────────
// Publicaciones
// ─────────────────────────────────────────────────────────

func TestCreatePostUsaEncabezadoComoAutor(t *testing.T) {
	var gotUser, gotContent string
	p := &fakePosts{createFn: func(userID, content string) (*postspb.Post, error) {
		gotUser, gotContent = userID, content
		return &postspb.Post{PostId: "p-9", UserId: userID, Content: content,
			CreatedAt: timestamppb.New(time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC))}, nil
	}}
	rec := do(t, newTestRouterWith(t, testBackends{posts: p}), http.MethodPost, "/posts",
		`{"content":"hola kite"}`, map[string]string{"X-User-Id": "u-1"})

	if rec.Code != http.StatusCreated {
		t.Fatalf("esperaba 201, obtuve %d (%s)", rec.Code, rec.Body.String())
	}
	if gotUser != "u-1" || gotContent != "hola kite" {
		t.Errorf("argumentos mal resueltos: user=%q content=%q", gotUser, gotContent)
	}
	body := decodeBody(t, rec)
	if body["post_id"] != "p-9" || body["status"] != "created" || body["created_at"] == nil {
		t.Errorf("cuerpo inesperado: %v", body)
	}
}

func TestCreatePostSinEncabezadoResponde400(t *testing.T) {
	rec := do(t, newTestRouter(t, nil, nil, nil), http.MethodPost, "/posts", `{"content":"x"}`, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperaba 400, obtuve %d", rec.Code)
	}
	if body := decodeBody(t, rec); body["error"] != "missing_user" {
		t.Errorf("código de error inesperado: %v", body)
	}
}

func TestGetPostDevuelveElContrato(t *testing.T) {
	rec := do(t, newTestRouter(t, nil, nil, nil), http.MethodGet, "/posts/p-5", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba 200, obtuve %d (%s)", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec)
	if body["post_id"] != "p-5" || body["user_id"] != "author-1" || body["content"] == nil {
		t.Errorf("cuerpo inesperado: %v", body)
	}
}

func TestGetPostNoEncontradoResponde404(t *testing.T) {
	p := &fakePosts{getFn: func(string) (*postspb.Post, error) {
		return nil, status.Error(codes.NotFound, "publicación no encontrada")
	}}
	rec := do(t, newTestRouterWith(t, testBackends{posts: p}), http.MethodGet, "/posts/p-9", "", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("esperaba 404, obtuve %d", rec.Code)
	}
	if body := decodeBody(t, rec); body["message"] != "publicación no encontrada" {
		t.Errorf("mensaje no propagado: %v", body)
	}
}

// ─────────────────────────────────────────────────────────
// Reacciones
// ─────────────────────────────────────────────────────────

func TestLikeResuelveElAutorConPosts(t *testing.T) {
	p := &fakePosts{getFn: func(postID string) (*postspb.Post, error) {
		return &postspb.Post{PostId: postID, UserId: "author-42", Content: "x"}, nil
	}}
	l := &fakeLikes{likeFn: func(postID, userID, postAuthorID string) (string, string, error) {
		return "like-1", "ok", nil
	}}
	rec := do(t, newTestRouterWith(t, testBackends{posts: p, likes: l}), http.MethodPost,
		"/posts/p-7/like", "", map[string]string{"X-User-Id": "u-1"})

	if rec.Code != http.StatusCreated {
		t.Fatalf("esperaba 201, obtuve %d (%s)", rec.Code, rec.Body.String())
	}
	if l.gotAuthor != "author-42" {
		t.Errorf("el gateway no resolvió el autor: %q", l.gotAuthor)
	}
	body := decodeBody(t, rec)
	if body["like_id"] != "like-1" || body["status"] != "ok" {
		t.Errorf("cuerpo inesperado: %v", body)
	}
}

func TestLikeSinEncabezadoResponde400(t *testing.T) {
	rec := do(t, newTestRouter(t, nil, nil, nil), http.MethodPost, "/posts/p-7/like", "", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperaba 400, obtuve %d", rec.Code)
	}
}

// Un post inexistente debe cortar en Posts: ni siquiera se llama a Likes.
func TestLikePostInexistenteNoLlamaALikes(t *testing.T) {
	called := false
	p := &fakePosts{getFn: func(string) (*postspb.Post, error) {
		return nil, status.Error(codes.NotFound, "publicación no encontrada")
	}}
	l := &fakeLikes{likeFn: func(string, string, string) (string, string, error) {
		called = true
		return "", "", nil
	}}
	rec := do(t, newTestRouterWith(t, testBackends{posts: p, likes: l}), http.MethodPost,
		"/posts/p-x/like", "", map[string]string{"X-User-Id": "u-1"})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("esperaba 404, obtuve %d", rec.Code)
	}
	if called {
		t.Error("Likes no debía ser llamado si el post no existe")
	}
}

func TestUnlikeResponde204(t *testing.T) {
	rec := do(t, newTestRouter(t, nil, nil, nil), http.MethodDelete, "/posts/p-7/like", "",
		map[string]string{"X-User-Id": "u-1"})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("esperaba 204, obtuve %d", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("204 no debe llevar cuerpo, lleva %q", rec.Body.String())
	}
}

func TestUnlikePropagaNotFound(t *testing.T) {
	l := &fakeLikes{unlikeFn: func(string, string) error {
		return status.Error(codes.NotFound, "like no encontrado")
	}}
	rec := do(t, newTestRouterWith(t, testBackends{likes: l}), http.MethodDelete,
		"/posts/p-7/like", "", map[string]string{"X-User-Id": "u-1"})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("esperaba 404, obtuve %d", rec.Code)
	}
}

// ─────────────────────────────────────────────────────────
// Feed (agregación)
// ─────────────────────────────────────────────────────────

func TestFeedAgregaFeedYPosts(t *testing.T) {
	f := &fakeFeed{userFeedFn: func(userID string, limit, offset int32) ([]string, int32, string, error) {
		return []string{"p-2", "p-1"}, 7, "2026-10-18T12:00:00Z", nil
	}}
	p := &fakePosts{getByIDsFn: func(ids []string) ([]*postspb.Post, error) {
		// Posts responde fuera de orden: el orden del muro manda.
		return []*postspb.Post{
			{PostId: "p-1", UserId: "a", Content: "primero"},
			{PostId: "p-2", UserId: "b", Content: "segundo"},
		}, nil
	}}
	rec := do(t, newTestRouterWith(t, testBackends{feed: f, posts: p}), http.MethodGet,
		"/feed?limit=2&offset=0", "", map[string]string{"X-User-Id": "u-1"})

	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba 200, obtuve %d (%s)", rec.Code, rec.Body.String())
	}
	if f.gotPagination.limit != 2 || f.gotPagination.offset != 0 {
		t.Errorf("paginación mal reenviada: %+v", f.gotPagination)
	}
	body := decodeBody(t, rec)
	if body["total"].(float64) != 7 {
		t.Errorf("total inesperado: %v", body["total"])
	}
	if body["last_updated"] != "2026-10-18T12:00:00Z" {
		t.Errorf("last_updated inesperado: %v", body["last_updated"])
	}
	posts := body["posts"].([]any)
	if len(posts) != 2 {
		t.Fatalf("esperaba 2 posts, obtuve %d", len(posts))
	}
	first := posts[0].(map[string]any)
	if first["post_id"] != "p-2" || first["content"] != "segundo" {
		t.Errorf("el orden del muro no se preservó: %v", first)
	}
}

func TestFeedSinUsuarioResponde400(t *testing.T) {
	rec := do(t, newTestRouter(t, nil, nil, nil), http.MethodGet, "/feed", "", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperaba 400, obtuve %d", rec.Code)
	}
}

// Un post borrado no debe romper la lectura del resto del muro.
func TestFeedOmitePostsDesaparecidos(t *testing.T) {
	f := &fakeFeed{userFeedFn: func(string, int32, int32) ([]string, int32, string, error) {
		return []string{"p-1", "p-borrado", "p-3"}, 3, "", nil
	}}
	p := &fakePosts{getByIDsFn: func(ids []string) ([]*postspb.Post, error) {
		return []*postspb.Post{
			{PostId: "p-1", UserId: "a", Content: "uno"},
			{PostId: "p-3", UserId: "a", Content: "tres"},
		}, nil
	}}
	rec := do(t, newTestRouterWith(t, testBackends{feed: f, posts: p}), http.MethodGet,
		"/feed", "", map[string]string{"X-User-Id": "u-1"})

	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba 200 pese al post desaparecido, obtuve %d", rec.Code)
	}
	posts := decodeBody(t, rec)["posts"].([]any)
	if len(posts) != 2 {
		t.Errorf("esperaba 2 posts legibles, obtuve %d", len(posts))
	}
}

func TestFeedVacioNoEsNull(t *testing.T) {
	f := &fakeFeed{userFeedFn: func(string, int32, int32) ([]string, int32, string, error) {
		return nil, 0, "", nil
	}}
	rec := do(t, newTestRouterWith(t, testBackends{feed: f}), http.MethodGet,
		"/feed", "", map[string]string{"X-User-Id": "u-1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba 200, obtuve %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"posts":[]`) {
		t.Errorf("esperaba lista vacía, obtuve %s", rec.Body.String())
	}
}

func TestFeedResponde503SiFeedCaido(t *testing.T) {
	f := &fakeFeed{userFeedFn: func(string, int32, int32) ([]string, int32, string, error) {
		return nil, 0, "", status.Error(codes.Unavailable, "connection refused")
	}}
	rec := do(t, newTestRouterWith(t, testBackends{feed: f}), http.MethodGet,
		"/feed", "", map[string]string{"X-User-Id": "u-1"})
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("esperaba 503, obtuve %d", rec.Code)
	}
}

func TestFeedResponde503SiPostsCaido(t *testing.T) {
	p := &fakePosts{getByIDsFn: func([]string) ([]*postspb.Post, error) {
		return nil, status.Error(codes.Unavailable, "connection refused")
	}}
	rec := do(t, newTestRouterWith(t, testBackends{posts: p}), http.MethodGet,
		"/feed", "", map[string]string{"X-User-Id": "u-1"})
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("esperaba 503, obtuve %d", rec.Code)
	}
}

// strPtr es un atajo para los campos opcionales.
func strPtr(s string) *string { return &s }
