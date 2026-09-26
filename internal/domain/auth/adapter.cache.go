package auth

import "github.com/esc-chula/intania-888-backend/pkg/cache"

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
	return r.cache.DeleteValue(key)
}

func (r *authRepositoryImpl) DeleteCacheValues(keys ...string) error {
	return r.cache.DeleteValues(keys...)
}

func (r *authRepositoryImpl) ConsumeCacheValue(key string, value interface{}) error {
	return r.cache.ConsumeValue(key, value)
}

func (r *authRepositoryImpl) CompareAndSwapCacheValues(expected map[string]interface{}, replacements map[string]interface{}, ttl int) (bool, error) {
	return r.cache.CompareAndSwapValues(expected, replacements, ttl)
}
