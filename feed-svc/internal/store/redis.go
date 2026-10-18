// Package store implementa el almacén sobre Redis. El muro de cada
// usuario es un sorted set en la clave feed:{user_id}: el miembro es
// el post_id y el score es la fecha de creación en epoch, que define
// el orden del muro. Feed es el único escritor de feed_db.
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const keyPrefix = "feed:"

// Redis implementa FeedStore sobre un cliente go-redis.
type Redis struct {
	rdb *redis.Client
}

// NewRedis conecta y verifica la base con un ping. La DSN tiene forma
// redis://usuario:clave@host:puerto/base.
func NewRedis(ctx context.Context, dsn string) (*Redis, error) {
	opts, err := redis.ParseURL(dsn)
	if err != nil {
		return nil, fmt.Errorf("store: DSN de Redis inválida: %w", err)
	}
	rdb := redis.NewClient(opts)
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, err
	}
	return &Redis{rdb: rdb}, nil
}

// Add reparte el post entre los muros indicados con una sola fuga de
// ida (pipeline): cada clave queda con el score de la publicación y,
// si hay TTL, con la vigencia renovada.
func (r *Redis) Add(ctx context.Context, userIDs []string, postID string, createdAt time.Time, ttl time.Duration) error {
	if len(userIDs) == 0 {
		return nil
	}
	score := float64(createdAt.Unix())
	pipe := r.rdb.Pipeline()
	for _, userID := range userIDs {
		key := keyPrefix + userID
		pipe.ZAdd(ctx, key, redis.Z{Score: score, Member: postID})
		if ttl > 0 {
			pipe.Expire(ctx, key, ttl)
		}
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("store: no se pudo actualizar el muro: %w", err)
	}
	return nil
}

// Page devuelve una página del muro de más reciente a más antiguo
// (ZREVRANGEBYSCORE) junto al total real (ZCARD), que es independiente
// de la página.
func (r *Redis) Page(ctx context.Context, userID string, limit, offset int32) ([]EntryRecord, int32, error) {
	key := keyPrefix + userID
	vals, err := r.rdb.ZRevRangeByScoreWithScores(ctx, key, &redis.ZRangeBy{
		Min:    "-inf",
		Max:    "+inf",
		Offset: int64(offset),
		Count:  int64(limit),
	}).Result()
	if err != nil {
		return nil, 0, fmt.Errorf("store: no se pudo leer el muro: %w", err)
	}
	total, err := r.rdb.ZCard(ctx, key).Result()
	if err != nil {
		return nil, 0, fmt.Errorf("store: no se pudo contar el muro: %w", err)
	}
	recs := make([]EntryRecord, 0, len(vals))
	for _, z := range vals {
		member, ok := z.Member.(string)
		if !ok {
			continue
		}
		recs = append(recs, EntryRecord{
			PostID:    member,
			CreatedAt: time.Unix(int64(z.Score), 0).UTC(),
		})
	}
	return recs, int32(total), nil
}

// Ping verifica la conectividad con Redis.
func (r *Redis) Ping(ctx context.Context) error { return r.rdb.Ping(ctx).Err() }

// Close libera la conexión con Redis.
func (r *Redis) Close() error { return r.rdb.Close() }
