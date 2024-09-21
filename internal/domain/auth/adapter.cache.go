package auth

import "github.com/wiraphatys/intania888/pkg/cache"

type authRepositoryImpl struct {
	cache cache.RedisClient
}

func NewAuthRepository(cache cache.RedisClient) AuthRepository {
	return &authRepositoryImpl{
		cache: cache,
	}
}

func (r *authRepositoryImpl) SetCacheValue(key string, value interface{}, ttl int) error {
	return r.cache.SetValue(key, value, ttl)
}

func (r *authRepositoryImpl) GetCacheValue(key string, value interface{}) error {
	return r.cache.GetValue(key, value)
}

func (r *authRepositoryImpl) DeleteCacheValue(key string) error {
	return nil
}
