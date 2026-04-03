package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/safrizal997/travapp-auth-service/internal/domain/repository"
)

type TokenRepo struct {
	client *redis.Client
}

func NewTokenRepo(client *redis.Client) *TokenRepo {
	return &TokenRepo{client: client}
}

func (r *TokenRepo) StoreRefreshToken(ctx context.Context, credentialID uuid.UUID, jti string, data *repository.RefreshTokenData, ttl time.Duration) error {
	key := fmt.Sprintf("rt:%s:%s", credentialID.String(), jti)
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal refresh token data: %w", err)
	}
	return r.client.Set(ctx, key, jsonData, ttl).Err()
}

func (r *TokenRepo) GetRefreshToken(ctx context.Context, credentialID uuid.UUID, jti string) (*repository.RefreshTokenData, error) {
	key := fmt.Sprintf("rt:%s:%s", credentialID.String(), jti)
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get refresh token: %w", err)
	}

	var data repository.RefreshTokenData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal refresh token data: %w", err)
	}
	return &data, nil
}

func (r *TokenRepo) DeleteRefreshToken(ctx context.Context, credentialID uuid.UUID, jti string) error {
	key := fmt.Sprintf("rt:%s:%s", credentialID.String(), jti)
	return r.client.Del(ctx, key).Err()
}

func (r *TokenRepo) DeleteAllRefreshTokens(ctx context.Context, credentialID uuid.UUID) error {
	pattern := fmt.Sprintf("rt:%s:*", credentialID.String())
	iter := r.client.Scan(ctx, 0, pattern, 100).Iterator()

	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return fmt.Errorf("failed to scan refresh tokens: %w", err)
	}

	if len(keys) > 0 {
		return r.client.Del(ctx, keys...).Err()
	}
	return nil
}

func (r *TokenRepo) FindRefreshTokenByHash(ctx context.Context, tokenHash string) (*repository.RefreshTokenData, string, string, error) {
	pattern := "rt:*"
	iter := r.client.Scan(ctx, 0, pattern, 100).Iterator()

	for iter.Next(ctx) {
		key := iter.Val()
		val, err := r.client.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var data repository.RefreshTokenData
		if err := json.Unmarshal([]byte(val), &data); err != nil {
			continue
		}

		if data.TokenHash == tokenHash {
			parts := strings.SplitN(key, ":", 3)
			if len(parts) == 3 {
				return &data, parts[1], parts[2], nil
			}
		}
	}

	return nil, "", "", nil
}

func (r *TokenRepo) DeleteRefreshTokenFamily(ctx context.Context, familyID string) error {
	pattern := "rt:*"
	iter := r.client.Scan(ctx, 0, pattern, 100).Iterator()

	var keysToDelete []string
	for iter.Next(ctx) {
		key := iter.Val()
		val, err := r.client.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var data repository.RefreshTokenData
		if err := json.Unmarshal([]byte(val), &data); err != nil {
			continue
		}

		if data.FamilyID == familyID {
			keysToDelete = append(keysToDelete, key)
		}
	}

	if len(keysToDelete) > 0 {
		return r.client.Del(ctx, keysToDelete...).Err()
	}
	return nil
}

func (r *TokenRepo) BlacklistAccessToken(ctx context.Context, jti string, ttl time.Duration) error {
	key := fmt.Sprintf("blacklist:at:%s", jti)
	return r.client.Set(ctx, key, "1", ttl).Err()
}

func (r *TokenRepo) IsAccessTokenBlacklisted(ctx context.Context, jti string) (bool, error) {
	key := fmt.Sprintf("blacklist:at:%s", jti)
	val, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check blacklist: %w", err)
	}
	return val > 0, nil
}

func (r *TokenRepo) StoreOAuthState(ctx context.Context, state string, data *repository.OAuthStateData, ttl time.Duration) error {
	key := fmt.Sprintf("oauth_state:%s", state)
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal OAuth state data: %w", err)
	}
	return r.client.Set(ctx, key, jsonData, ttl).Err()
}

func (r *TokenRepo) GetOAuthState(ctx context.Context, state string) (*repository.OAuthStateData, error) {
	key := fmt.Sprintf("oauth_state:%s", state)
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get OAuth state: %w", err)
	}

	var data repository.OAuthStateData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal OAuth state data: %w", err)
	}
	return &data, nil
}

func (r *TokenRepo) DeleteOAuthState(ctx context.Context, state string) error {
	key := fmt.Sprintf("oauth_state:%s", state)
	return r.client.Del(ctx, key).Err()
}

func (r *TokenRepo) IncrementRateLimit(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	script := redis.NewScript(`
		local current = redis.call('INCR', KEYS[1])
		if current == 1 then
			redis.call('EXPIRE', KEYS[1], ARGV[1])
		end
		return current
	`)
	result, err := script.Run(ctx, r.client, []string{key}, int(ttl.Seconds())).Int64()
	if err != nil {
		return 0, fmt.Errorf("failed to increment rate limit: %w", err)
	}
	return result, nil
}

func (r *TokenRepo) GetRateLimit(ctx context.Context, key string) (int64, error) {
	val, err := r.client.Get(ctx, key).Int64()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to get rate limit: %w", err)
	}
	return val, nil
}

type rbacCache struct {
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
}

func (r *TokenRepo) StoreRBACCache(ctx context.Context, credentialID uuid.UUID, role string, permissions []string, ttl time.Duration) error {
	key := fmt.Sprintf("cache:rbac:%s", credentialID.String())
	data := rbacCache{Role: role, Permissions: permissions}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal RBAC cache: %w", err)
	}
	return r.client.Set(ctx, key, jsonData, ttl).Err()
}

func (r *TokenRepo) GetRBACCache(ctx context.Context, credentialID uuid.UUID) (string, []string, error) {
	key := fmt.Sprintf("cache:rbac:%s", credentialID.String())
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil, nil
		}
		return "", nil, fmt.Errorf("failed to get RBAC cache: %w", err)
	}

	var data rbacCache
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return "", nil, fmt.Errorf("failed to unmarshal RBAC cache: %w", err)
	}
	return data.Role, data.Permissions, nil
}
