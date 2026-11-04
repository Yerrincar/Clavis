package store

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func requireStoreValue(t *testing.T, store *Store, key, want string) {
	t.Helper()

	value, exists := store.Get(key)
	if !exists {
		t.Fatalf("Get(%q) did not find a value", key)
	}
	if got := string(value.BorrowBytes()); got != want {
		t.Fatalf("Get(%q) = %q, want %q", key, got, want)
	}
}

func expireStoreKey(t *testing.T, store *Store, key, value string) {
	t.Helper()

	if err := store.Set(key, []byte(value), time.Hour); err != nil {
		t.Fatalf("Set(%q) error = %v", key, err)
	}

	store.mu.Lock()
	store.expire.expireData[key] = time.Now().Add(-time.Millisecond).UnixMilli()
	store.mu.Unlock()
}

func requireStoreKeyRemoved(t *testing.T, store *Store, key string) {
	t.Helper()

	store.mu.Lock()
	defer store.mu.Unlock()

	if _, exists := store.data[key]; exists {
		t.Fatalf("Store retained expired value for %q", key)
	}
	if _, exists := store.expire.expireData[key]; exists {
		t.Fatalf("Store retained expired metadata for %q", key)
	}
}

func TestStoreBasicStringCommands(t *testing.T) {
	store := NewStore()
	input := []byte("hello")

	if err := store.Set("key", input, 0); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	input[0] = 'X'
	requireStoreValue(t, store, "key", "hello")

	if got := store.Strlen("key"); got != 5 {
		t.Fatalf("Strlen() = %d, want 5", got)
	}
	if got := store.Exists([]string{"key", "missing", "key"}); got != 2 {
		t.Fatalf("Exists() = %d, want 2", got)
	}

	if got, err := store.Append("key", []byte("!")); err != nil || got != 6 {
		t.Fatalf("Append(existing) = (%d, %v), want (6, nil)", got, err)
	}
	if got, err := store.Append("new", []byte("abc")); err != nil || got != 3 {
		t.Fatalf("Append(missing) = (%d, %v), want (3, nil)", got, err)
	}
	requireStoreValue(t, store, "key", "hello!")
	requireStoreValue(t, store, "new", "abc")

	if got := store.Del([]string{"key", "missing", "key"}); got != 1 {
		t.Fatalf("Del() = %d, want 1", got)
	}
	if _, exists := store.Get("key"); exists {
		t.Fatal("Del() left key in Store")
	}
}

func TestStoreCounters(t *testing.T) {
	store := NewStore()

	if got, err := store.Incr("counter"); err != nil || got != 1 {
		t.Fatalf("Incr(missing) = (%d, %v), want (1, nil)", got, err)
	}
	if got, err := store.Decr("counter"); err != nil || got != 0 {
		t.Fatalf("Decr() = (%d, %v), want (0, nil)", got, err)
	}
	if got, err := store.Decr("missing"); err != nil || got != -1 {
		t.Fatalf("Decr(missing) = (%d, %v), want (-1, nil)", got, err)
	}

	if err := store.Set("invalid", []byte("not-an-integer"), 0); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if _, err := store.Incr("invalid"); !errors.Is(err, ErrInvalidInteger) {
		t.Fatalf("Incr(invalid) error = %v, want %v", err, ErrInvalidInteger)
	}
}

func TestStoreReadersRemainSafeDuringUpdates(t *testing.T) {
	store := NewStore()
	if err := store.Set("text", []byte("a"), 0); err != nil {
		t.Fatalf("Set(text) error = %v", err)
	}
	if err := store.Set("counter", []byte("0"), 0); err != nil {
		t.Fatalf("Set(counter) error = %v", err)
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		<-start
		for range 1000 {
			for _, key := range []string{"text", "counter"} {
				value, exists := store.Get(key)
				if !exists {
					t.Errorf("Get(%q) did not find a value", key)
					return
				}
				for range value.BorrowBytes() {
				}
			}
		}
	}()

	go func() {
		defer wg.Done()
		<-start
		for range 1000 {
			if _, err := store.Append("text", []byte("x")); err != nil {
				t.Errorf("Append() error = %v", err)
				return
			}
			if _, err := store.Incr("counter"); err != nil {
				t.Errorf("Incr() error = %v", err)
				return
			}
			if _, err := store.Decr("counter"); err != nil {
				t.Errorf("Decr() error = %v", err)
				return
			}
		}
	}()

	close(start)
	wg.Wait()
}

func TestStoreExpiredReadCommandsBehaveAsMissing(t *testing.T) {
	store := NewStore()

	expireStoreKey(t, store, "get", "value")
	if _, exists := store.Get("get"); exists {
		t.Fatal("Get() found an expired key")
	}
	requireStoreKeyRemoved(t, store, "get")

	expireStoreKey(t, store, "exists", "value")
	if got := store.Exists([]string{"exists"}); got != 0 {
		t.Fatalf("Exists() = %d, want 0 for expired key", got)
	}
	requireStoreKeyRemoved(t, store, "exists")

	expireStoreKey(t, store, "length", "value")
	if got := store.Strlen("length"); got != 0 {
		t.Fatalf("Strlen() = %d, want 0 for expired key", got)
	}
	requireStoreKeyRemoved(t, store, "length")

	expireStoreKey(t, store, "delete", "value")
	if got := store.Del([]string{"delete"}); got != 0 {
		t.Fatalf("Del() = %d, want 0 for expired key", got)
	}
	requireStoreKeyRemoved(t, store, "delete")
}

func TestStoreExpiredMutationCommandsCreateNewValues(t *testing.T) {
	store := NewStore()

	expireStoreKey(t, store, "append", "old")
	if got, err := store.Append("append", []byte("new")); err != nil || got != 3 {
		t.Fatalf("Append() = (%d, %v), want (3, nil)", got, err)
	}
	requireStoreValue(t, store, "append", "new")

	expireStoreKey(t, store, "incr", "41")
	if got, err := store.Incr("incr"); err != nil || got != 1 {
		t.Fatalf("Incr() = (%d, %v), want (1, nil)", got, err)
	}
	requireStoreValue(t, store, "incr", "1")

	expireStoreKey(t, store, "decr", "41")
	if got, err := store.Decr("decr"); err != nil || got != -1 {
		t.Fatalf("Decr() = (%d, %v), want (-1, nil)", got, err)
	}
	requireStoreValue(t, store, "decr", "-1")
}

func TestStoreSetWithoutTTLClearsExpiration(t *testing.T) {
	store := NewStore()
	if err := store.Set("key", []byte("temporary"), time.Hour); err != nil {
		t.Fatalf("Set(temporary) error = %v", err)
	}
	if _, hasExpiry := store.expire.expireData["key"]; !hasExpiry {
		t.Fatal("Set() with TTL did not store expiration metadata")
	}
	if err := store.Set("key", []byte("persistent"), 0); err != nil {
		t.Fatalf("Set(persistent) error = %v", err)
	}

	store.mu.Lock()
	_, hasExpiry := store.expire.expireData["key"]
	store.mu.Unlock()
	if hasExpiry {
		t.Fatal("Set() without TTL retained the previous expiration")
	}
	requireStoreValue(t, store, "key", "persistent")
}

func TestStoreGetExpiresKey(t *testing.T) {
	store := NewStore()
	if err := store.Set("key", []byte("value"), 20*time.Millisecond); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	requireStoreValue(t, store, "key", "value")

	time.Sleep(40 * time.Millisecond)
	if _, exists := store.Get("key"); exists {
		t.Fatal("Get() found an expired key")
	}
	requireStoreKeyRemoved(t, store, "key")
}

func TestStoreActiveExpiryRemovesBothMaps(t *testing.T) {
	store := NewStore()
	expireStoreKey(t, store, "key", "value")

	store.activeExpireCycle(time.Millisecond)

	requireStoreKeyRemoved(t, store, "key")
}
