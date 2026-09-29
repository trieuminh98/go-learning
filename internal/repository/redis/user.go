package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"go-learning/internal/core"
	"go-learning/internal/logic"
)

var _ logic.UserCache = (*UserCache)(nil)

// cachedUserDTO encapsulates the serialized representation of core.User for cache storage.
// It keeps transport/storage-specific JSON tags out of the pure domain entity (core.User).
type cachedUserDTO struct {
	ID           string `json:"id"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Email        string `json:"email"`
	PasswordHash []byte `json:"password_hash,omitempty"`
	IsActive     bool   `json:"is_active"`
	RoleID       int64  `json:"role_id"`
	RoleName     string `json:"role_name"`
	RoleLevel    int    `json:"role_level"`
	RoleDesc     string `json:"role_desc"`
}

// UserCache implements logic.UserCache using Redis.
type UserCache struct {
	client *goredis.Client
	logger *zap.SugaredLogger
}

// NewUserCache constructs a new Redis-backed UserCache.
func NewUserCache(client *goredis.Client, logger *zap.SugaredLogger) *UserCache {
	return &UserCache{
		client: client,
		logger: logger,
	}
}

// userKey formats the standard namespaced Redis key for user entities: "user:{id}".
func (c *UserCache) userKey(id int64) string {
	return fmt.Sprintf("user:%d", id)
}

// Get attempts to read and deserialize a cached user by ID.
// If the key does not exist (redis.Nil), it returns (nil, nil) representing a cache miss.
// If a Redis or deserialization error occurs, it returns an error so the caller can log and fallback.
func (c *UserCache) Get(ctx context.Context, userID int64) (*core.User, error) {
	if c.client == nil {
		return nil, nil
	}

	key := c.userKey(userID)
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, nil // Cache miss is not an error
		}
		if c.logger != nil {
			c.logger.Warnw("redis get error, falling back to db", "key", key, "error", err)
		}
		return nil, err
	}

	var dto cachedUserDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		if c.logger != nil {
			c.logger.Warnw("failed to unmarshal cached user json, invalidating corrupted entry", "key", key, "error", err)
		}
		// Best effort clean up corrupted entry
		_ = c.client.Del(ctx, key)
		return nil, err
	}

	user := core.User{
		ID:        dto.ID,
		FirstName: dto.FirstName,
		LastName:  dto.LastName,
		Email:     dto.Email,
		Password:  core.NewPasswordFromHash(dto.PasswordHash),
		IsActive:  dto.IsActive,
		Role: core.Role{
			ID:          dto.RoleID,
			Name:        dto.RoleName,
			Level:       dto.RoleLevel,
			Description: dto.RoleDesc,
		},
	}

	return &user, nil
}

// Set serializes and stores a user entity into Redis with the given TTL.
func (c *UserCache) Set(ctx context.Context, user *core.User, ttl time.Duration) error {
	if c.client == nil || user == nil {
		return nil
	}

	id, err := strconv.ParseInt(user.ID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid user id format for cache key: %w", err)
	}

	key := c.userKey(id)
	dto := cachedUserDTO{
		ID:           user.ID,
		FirstName:    user.FirstName,
		LastName:     user.LastName,
		Email:        user.Email,
		PasswordHash: user.Password.Hash(),
		IsActive:     user.IsActive,
		RoleID:       user.Role.ID,
		RoleName:     user.Role.Name,
		RoleLevel:    user.Role.Level,
		RoleDesc:     user.Role.Description,
	}

	data, err := json.Marshal(dto)
	if err != nil {
		return fmt.Errorf("failed to marshal user for cache: %w", err)
	}

	if err := c.client.Set(ctx, key, data, ttl).Err(); err != nil {
		if c.logger != nil {
			c.logger.Warnw("redis set error", "key", key, "error", err)
		}
		return err
	}

	return nil
}

// Delete removes a cached user entry by ID (used for cache invalidation).
func (c *UserCache) Delete(ctx context.Context, userID int64) error {
	if c.client == nil {
		return nil
	}

	key := c.userKey(userID)
	if err := c.client.Del(ctx, key).Err(); err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil
		}
		if c.logger != nil {
			c.logger.Warnw("redis del error during invalidation", "key", key, "error", err)
		}
		return err
	}

	return nil
}
