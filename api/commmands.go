package api

import "Clavis/src/store"

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

func (c *Cache) Set(key string, value []byte) error {
	err := c.store.Set(key, value)
	if err != nil {
		return err
	}

	return nil
}
