package api

import (
	"testing"

	"Clavis/src/store"
)

func TestCacheGetReturnsCopy(t *testing.T) {
	backend := store.NewStore()
	if err := backend.Set("key", []byte("value")); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	cache := Cache{store: backend}

	value, exists := cache.Get("key")
	if !exists {
		t.Fatal("Get() did not find a value")
	}
	value[0] = 'X'

	value, exists = cache.Get("key")
	if !exists || string(value) != "value" {
		t.Fatalf("Get() after caller mutation = %q, %t; want value, true", value, exists)
	}
}
