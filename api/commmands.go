package api

import (
	"Clavis/src/store"
	"time"
)

type Cache struct {
	store *store.Store
}

func NewCache() *Cache {
	return &Cache{store: store.NewStore()}
}

func (c *Cache) Get(key string) ([]byte, bool) {
	value, exists := c.store.Get(key)
	if !exists {
		return nil, false
	}

	return value.CloneBytes(), true
}

func (c *Cache) Set(key string, value []byte, ttl time.Duration) error {
	err := c.store.Set(key, value, ttl)
	if err != nil {
		return err
	}

	return nil
}

func (c *Cache) Del(key ...string) int {
	keysDel := c.store.Del(key)
	return keysDel
}
