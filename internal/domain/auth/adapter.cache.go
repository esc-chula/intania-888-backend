package auth

import "github.com/esc-chula/intania-888-backend/pkg/cache"

type authRepositoryImpl struct{ cache cache.RedisClient }

func NewAuthRepository(c cache.RedisClient) AuthRepository {
	return &authRepositoryImpl{cache: c}
}

func (r *authRepositoryImpl) SetCacheValue(k string, v interface{}, ttl int) error {
	return r.cache.SetValue(k, v, ttl)
}

func (r *authRepositoryImpl) ConsumeCacheValue(k string, v interface{}) error {
	return r.cache.ConsumeValue(k, v)
}

func (r *authRepositoryImpl) RotateSession(u, k, previous string, v interface{}, ttl int) error {
	return r.cache.RotateSession(u, k, previous, v, ttl)
}

func (r *authRepositoryImpl) DeleteSession(k string) error {
	return r.cache.DeleteSession(k)
}

func (r *authRepositoryImpl) GetCacheValue(k string, v interface{}) error {
	return r.cache.GetValue(k, v)
}
