// Package store implementa el almacén sobre MongoDB.
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type MongoDB struct {
	client *mongo.Client
	db     *mongo.Database
	posts  *mongo.Collection
}

const (
	dbName         = "posts_db"
	collectionName = "posts"
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
		posts:  client.Database(dbName).Collection(collectionName),
	}
	return m, nil
}

func (m *MongoDB) Migrate(ctx context.Context) error {
	_, _ = m.posts.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "created_at", Value: -1}}},
		{Keys: bson.D{{Key: "created_at", Value: -1}}},
	})
	return nil
}

func (m *MongoDB) Create(ctx context.Context, userID, content string) (PostRecord, error) {
	now := time.Now().UTC()
	id := bson.NewObjectID()
	rec := bson.M{
		"_id":        id,
		"user_id":    userID,
		"content":    content,
		"created_at": now,
	}
	if _, err := m.posts.InsertOne(ctx, rec); err != nil {
		return PostRecord{}, err
	}
	return PostRecord{ID: id.Hex(), UserID: userID, Content: content, CreatedAt: now}, nil
}

func (m *MongoDB) Get(ctx context.Context, id string) (PostRecord, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return PostRecord{}, ErrNotFound
	}
	var r bson.M
	if err := m.posts.FindOne(ctx, bson.M{"_id": oid}).Decode(&r); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return PostRecord{}, ErrNotFound
		}
		return PostRecord{}, err
	}
	return mapRecord(r)
}

func (m *MongoDB) GetByIDs(ctx context.Context, ids []string) ([]PostRecord, error) {
	if len(ids) == 0 {
		return []PostRecord{}, nil
	}
	oids := make([]bson.ObjectID, 0, len(ids))
	for _, s := range ids {
		if oid, err := bson.ObjectIDFromHex(s); err == nil {
			oids = append(oids, oid)
		}
	}
	cur, err := m.posts.Find(ctx, bson.M{"_id": bson.M{"$in": oids}})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	res := []PostRecord{}
	for cur.Next(ctx) {
		var r bson.M
		if err := cur.Decode(&r); err == nil {
			if pr, err := mapRecord(r); err == nil {
				res = append(res, pr)
			}
		}
	}
	return res, cur.Err()
}

func (m *MongoDB) GetByUser(ctx context.Context, userID string, limit, offset int32) ([]PostRecord, int32, error) {
	filter := bson.M{"user_id": userID}
	total, err := m.posts.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	if limit > 0 {
		opts.SetLimit(int64(limit))
	}
	if offset > 0 {
		opts.SetSkip(int64(offset))
	}
	cur, err := m.posts.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	res := []PostRecord{}
	for cur.Next(ctx) {
		var r bson.M
		if err := cur.Decode(&r); err == nil {
			if pr, err := mapRecord(r); err == nil {
				res = append(res, pr)
			}
		}
	}
	if err := cur.Err(); err != nil {
		return nil, 0, err
	}
	return res, int32(total), nil
}

func (m *MongoDB) Delete(ctx context.Context, id, userID string) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return ErrNotFound
	}
	res, err := m.posts.DeleteOne(ctx, bson.M{"_id": oid, "user_id": userID})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func (m *MongoDB) Ping(ctx context.Context) error {
	return m.client.Ping(ctx, nil)
}

func (m *MongoDB) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return m.client.Disconnect(ctx)
}

func mapRecord(r bson.M) (PostRecord, error) {
	id, ok := r["_id"]
	if !ok {
		return PostRecord{}, errors.New("sin _id")
	}
	var hex string
	switch v := id.(type) {
	case bson.ObjectID:
		hex = v.Hex()
	default:
		b, _ := bson.Marshal(id)
		var tmp bson.M
		_ = bson.Unmarshal(b, &tmp)
		if oid, ok2 := tmp["$oid"].(string); ok2 {
			hex = oid
		} else {
			hex = fmt.Sprint(v)
		}
	}
	pr := PostRecord{ID: hex}
	if s, ok := r["user_id"].(string); ok {
		pr.UserID = s
	}
	if s, ok := r["content"].(string); ok {
		pr.Content = s
	}
	if t, ok := r["created_at"].(time.Time); ok {
		pr.CreatedAt = t
	}
	return pr, nil
}
