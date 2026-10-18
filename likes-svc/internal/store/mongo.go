// Package store implementa el almacén sobre MongoDB. La colección
// likes tiene un índice único (post_id, user_id) que garantiza que un
// usuario reaccione una sola vez a cada publicación.
package store

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type MongoDB struct {
	client *mongo.Client
	db     *mongo.Database
	likes  *mongo.Collection
}

const (
	dbName         = "likes_db"
	collectionName = "likes"
)

func NewMongo(ctx context.Context, dsn string) (*MongoDB, error) {
	opts := options.Client().ApplyURI(dsn)
	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.WithoutCancel(ctx))
		return nil, err
	}
	m := &MongoDB{
		client: client,
		db:     client.Database(dbName),
		likes:  client.Database(dbName).Collection(collectionName),
	}
	return m, nil
}

func (m *MongoDB) Migrate(ctx context.Context) error {
	_, _ = m.likes.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "post_id", Value: 1}, {Key: "user_id", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("likes_post_user"),
		},
		{Keys: bson.D{{Key: "post_id", Value: 1}, {Key: "created_at", Value: -1}}, Options: options.Index().SetName("likes_post_created")},
	})
	return nil
}

func (m *MongoDB) Create(ctx context.Context, postID, userID string) (LikeRecord, bool, error) {
	now := time.Now().UTC()
	id := bson.NewObjectID()
	rec := bson.M{
		"_id":        id,
		"post_id":    postID,
		"user_id":    userID,
		"created_at": now,
	}
	if _, err := m.likes.InsertOne(ctx, rec); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			// Ya existía: devolvemos el like original sin volver a insertar.
			var r bson.M
			if e := m.likes.FindOne(ctx, bson.M{"post_id": postID, "user_id": userID}).Decode(&r); e != nil {
				return LikeRecord{}, false, e
			}
			like, e := mapRecord(r)
			return like, false, e
		}
		return LikeRecord{}, false, err
	}
	return LikeRecord{ID: id.Hex(), PostID: postID, UserID: userID, CreatedAt: now}, true, nil
}

func (m *MongoDB) Delete(ctx context.Context, postID, userID string) error {
	res, err := m.likes.DeleteOne(ctx, bson.M{"post_id": postID, "user_id": userID})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func (m *MongoDB) ListByPost(ctx context.Context, postID string, limit, offset int32) ([]LikeRecord, int32, error) {
	filter := bson.M{"post_id": postID}
	total, err := m.likes.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}})
	if limit > 0 {
		opts.SetLimit(int64(limit))
	}
	if offset > 0 {
		opts.SetSkip(int64(offset))
	}
	cur, err := m.likes.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	res := []LikeRecord{}
	for cur.Next(ctx) {
		var r bson.M
		if err := cur.Decode(&r); err == nil {
			if like, err := mapRecord(r); err == nil {
				res = append(res, like)
			}
		}
	}
	if err := cur.Err(); err != nil {
		return nil, 0, err
	}
	return res, int32(total), nil
}

func (m *MongoDB) Exists(ctx context.Context, postID, userID string) (bool, error) {
	opts := options.FindOne().SetProjection(bson.M{"_id": 1})
	var r bson.M
	if err := m.likes.FindOne(ctx, bson.M{"post_id": postID, "user_id": userID}, opts).Decode(&r); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (m *MongoDB) Ping(ctx context.Context) error {
	return m.client.Ping(ctx, nil)
}

func (m *MongoDB) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return m.client.Disconnect(ctx)
}

func mapRecord(r bson.M) (LikeRecord, error) {
	id, ok := r["_id"]
	if !ok {
		return LikeRecord{}, errors.New("sin _id")
	}
	oid, ok := id.(bson.ObjectID)
	if !ok {
		return LikeRecord{}, errors.New("_id con formato inesperado")
	}
	rec := LikeRecord{ID: oid.Hex()}
	if s, ok := r["post_id"].(string); ok {
		rec.PostID = s
	}
	if s, ok := r["user_id"].(string); ok {
		rec.UserID = s
	}
	// El driver decodifica las fechas BSON como bson.DateTime (no como
	// time.Time), así que se convierte explícitamente.
	switch t := r["created_at"].(type) {
	case time.Time:
		rec.CreatedAt = t
	case bson.DateTime:
		rec.CreatedAt = t.Time()
	}
	return rec, nil
}
