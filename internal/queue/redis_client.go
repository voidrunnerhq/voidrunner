package queue

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/voidrunnerhq/voidrunner/internal/config"
)

// RedisClient wraps the Redis client with queue-specific functionality
type RedisClient struct {
	client *redis.Client
	config *config.RedisConfig
	logger *slog.Logger
}

// NewRedisClient creates a new Redis client for queue operations
func NewRedisClient(cfg *config.RedisConfig, logger *slog.Logger) (*RedisClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("redis config is required")
	}

	if logger == nil {
		logger = slog.Default()
	}

	// Create Redis client options
	options := &redis.Options{
		Addr:         fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
		DB:           cfg.Database,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConnections,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	// Only set password if it's not empty
	if cfg.Password != "" {
		options.Password = cfg.Password
	}

	client := redis.NewClient(options)

	redisClient := &RedisClient{
		client: client,
		config: cfg,
		logger: logger,
	}

	return redisClient, nil
}

// Ping tests the Redis connection
func (r *RedisClient) Ping(ctx context.Context) error {
	result := r.client.Ping(ctx)
	if result.Err() != nil {
		return NewQueueError("ping", result.Err(), true)
	}

	r.logger.Debug("Redis ping successful", "result", result.Val())
	return nil
}

// IsHealthy checks if the Redis connection is healthy
func (r *RedisClient) IsHealthy(ctx context.Context) error {
	// Test connection with ping
	if err := r.Ping(ctx); err != nil {
		return fmt.Errorf("redis health check failed: %w", err)
	}

	// Check pool stats
	stats := r.client.PoolStats()
	r.logger.Debug("Redis pool stats",
		"hits", stats.Hits,
		"misses", stats.Misses,
		"timeouts", stats.Timeouts,
		"total_conns", stats.TotalConns,
		"idle_conns", stats.IdleConns,
		"stale_conns", stats.StaleConns,
	)

	// Warn if pool is under stress
	// Use int64 comparison to avoid unsafe int to uint32 conversion
	if r.config.PoolSize > 0 && int64(stats.TotalConns) >= int64(r.config.PoolSize) {
		r.logger.Warn("Redis connection pool at capacity",
			"total_conns", stats.TotalConns,
			"pool_size", r.config.PoolSize,
		)
	}

	return nil
}

// GetClient returns the underlying Redis client
func (r *RedisClient) GetClient() *redis.Client {
	return r.client
}

// Close closes the Redis connection
func (r *RedisClient) Close() error {
	if r.client != nil {
		err := r.client.Close()
		if err != nil {
			r.logger.Error("failed to close Redis client", "error", err)
			return err
		}
		r.logger.Info("Redis client closed successfully")
	}
	return nil
}

// ZAddWithScore adds a member to a sorted set with a score
func (r *RedisClient) ZAddWithScore(ctx context.Context, key string, score float64, member interface{}) error {
	result := r.client.ZAdd(ctx, key, &redis.Z{
		Score:  score,
		Member: member,
	})

	if result.Err() != nil {
		return NewQueueError("zadd", result.Err(), true)
	}

	return nil
}

// ZRangeByScoreWithLimit retrieves members from a sorted set by score range with limit
func (r *RedisClient) ZRangeByScoreWithLimit(ctx context.Context, key string, min, max string, offset, count int64) ([]string, error) {
	result := r.client.ZRangeByScore(ctx, key, &redis.ZRangeBy{
		Min:    min,
		Max:    max,
		Offset: offset,
		Count:  count,
	})

	if result.Err() != nil {
		return nil, NewQueueError("zrangebyscore", result.Err(), true)
	}

	return result.Val(), nil
}

// ZRem removes members from a sorted set
func (r *RedisClient) ZRem(ctx context.Context, key string, members ...interface{}) error {
	result := r.client.ZRem(ctx, key, members...)

	if result.Err() != nil {
		return NewQueueError("zrem", result.Err(), true)
	}

	return nil
}

// ZCard returns the number of members in a sorted set
func (r *RedisClient) ZCard(ctx context.Context, key string) (int64, error) {
	result := r.client.ZCard(ctx, key)

	if result.Err() != nil {
		return 0, NewQueueError("zcard", result.Err(), true)
	}

	return result.Val(), nil
}

// HSet sets field-value pairs in a hash
func (r *RedisClient) HSet(ctx context.Context, key string, values ...interface{}) error {
	result := r.client.HSet(ctx, key, values...)

	if result.Err() != nil {
		return NewQueueError("hset", result.Err(), true)
	}

	return nil
}

// HGet gets a field value from a hash
func (r *RedisClient) HGet(ctx context.Context, key, field string) (string, error) {
	result := r.client.HGet(ctx, key, field)

	if result.Err() != nil {
		if result.Err() == redis.Nil {
			return "", nil // Field doesn't exist
		}
		return "", NewQueueError("hget", result.Err(), true)
	}

	return result.Val(), nil
}

// HGetAll gets all field-value pairs from a hash
func (r *RedisClient) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	result := r.client.HGetAll(ctx, key)

	if result.Err() != nil {
		return nil, NewQueueError("hgetall", result.Err(), true)
	}

	return result.Val(), nil
}

// HDel deletes fields from a hash
func (r *RedisClient) HDel(ctx context.Context, key string, fields ...string) error {
	result := r.client.HDel(ctx, key, fields...)

	if result.Err() != nil {
		return NewQueueError("hdel", result.Err(), true)
	}

	return nil
}

// Del deletes keys
func (r *RedisClient) Del(ctx context.Context, keys ...string) error {
	result := r.client.Del(ctx, keys...)

	if result.Err() != nil {
		return NewQueueError("del", result.Err(), true)
	}

	return nil
}

// Exists checks if keys exist
func (r *RedisClient) Exists(ctx context.Context, keys ...string) (int64, error) {
	result := r.client.Exists(ctx, keys...)

	if result.Err() != nil {
		return 0, NewQueueError("exists", result.Err(), true)
	}

	return result.Val(), nil
}

// Expire sets expiration for a key
func (r *RedisClient) Expire(ctx context.Context, key string, expiration time.Duration) error {
	result := r.client.Expire(ctx, key, expiration)

	if result.Err() != nil {
		return NewQueueError("expire", result.Err(), true)
	}

	return nil
}

// ExecuteLuaScript executes a Lua script
func (r *RedisClient) ExecuteLuaScript(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
	luaScript := redis.NewScript(script)
	result := luaScript.Run(ctx, r.client, keys, args...)

	if result.Err() != nil {
		return nil, NewQueueError("eval", result.Err(), true)
	}

	return result.Val(), nil
}

// Pipeline creates a new pipeline for batch operations
func (r *RedisClient) Pipeline() redis.Pipeliner {
	return r.client.Pipeline()
}

// ExecutePipeline executes a pipeline
func (r *RedisClient) ExecutePipeline(ctx context.Context, pipe redis.Pipeliner) error {
	_, err := pipe.Exec(ctx)
	if err != nil {
		return NewQueueError("pipeline", err, true)
	}
	return nil
}
