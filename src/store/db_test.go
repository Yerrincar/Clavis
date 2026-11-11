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

func requireLRUKey(t *testing.T, store *Store, key string, want bool) {
	t.Helper()

	store.mu.Lock()
	defer store.mu.Unlock()

	_, got := store.lruCache.Cache[key]
	if got != want {
		t.Fatalf("LRU key %q exists = %t, want %t", key, got, want)
	}
}

func newLimitedStore(maxBytes int64) *Store {
	store := NewStore()
	store.limit.MaxBytes = maxBytes
	return store
}

func requireStoreUsage(t *testing.T, store *Store, want int64) {
	t.Helper()

	store.mu.Lock()
	defer store.mu.Unlock()

	if got := store.actualUsage; got != want {
		t.Fatalf("actualUsage = %d, want %d", got, want)
	}
}

func requireStats(t *testing.T, store *Store, want Stats) {
	t.Helper()

	if got := store.StatsInfo(); got != want {
		t.Fatalf("StatsInfo() = %#v, want %#v", got, want)
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

func TestStoreSetMemoryLimitTracksReplacements(t *testing.T) {
	store := newLimitedStore(5)

	if err := store.Set("a", []byte("1234"), 0); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	requireStoreUsage(t, store, 5)

	if err := store.Set("a", []byte("12345"), 0); !errors.Is(err, ErrMemoryLimitSurpassed) {
		t.Fatalf("Set() error = %v, want %v", err, ErrMemoryLimitSurpassed)
	}
	requireStoreValue(t, store, "a", "1234")
	requireStoreUsage(t, store, 5)

	if err := store.Set("a", []byte("12"), 0); err != nil {
		t.Fatalf("Set(smaller) error = %v", err)
	}
	requireStoreUsage(t, store, 3)
}

func TestStoreMutationMemoryLimitsPreserveValues(t *testing.T) {
	t.Run("append", func(t *testing.T) {
		store := newLimitedStore(4)
		if err := store.Set("a", []byte("b"), 0); err != nil {
			t.Fatalf("Set() error = %v", err)
		}
		if _, err := store.Append("a", []byte("cde")); !errors.Is(err, ErrMemoryLimitSurpassed) {
			t.Fatalf("Append() error = %v, want %v", err, ErrMemoryLimitSurpassed)
		}
		requireStoreValue(t, store, "a", "b")
		requireStoreUsage(t, store, 2)
	})

	t.Run("incr", func(t *testing.T) {
		store := newLimitedStore(2)
		if err := store.Set("a", []byte("9"), 0); err != nil {
			t.Fatalf("Set() error = %v", err)
		}
		if _, err := store.Incr("a"); !errors.Is(err, ErrMemoryLimitSurpassed) {
			t.Fatalf("Incr() error = %v, want %v", err, ErrMemoryLimitSurpassed)
		}
		requireStoreValue(t, store, "a", "9")
		requireStoreUsage(t, store, 2)
	})

	t.Run("decr", func(t *testing.T) {
		store := newLimitedStore(2)
		if err := store.Set("a", []byte("0"), 0); err != nil {
			t.Fatalf("Set() error = %v", err)
		}
		if _, err := store.Decr("a"); !errors.Is(err, ErrMemoryLimitSurpassed) {
			t.Fatalf("Decr() error = %v, want %v", err, ErrMemoryLimitSurpassed)
		}
		requireStoreValue(t, store, "a", "0")
		requireStoreUsage(t, store, 2)
	})
}

func TestStoreSetEvictsLeastRecentlyUsedKey(t *testing.T) {
	store := newLimitedStore(4)

	if err := store.Set("a", []byte("1"), 0); err != nil {
		t.Fatalf("Set(a) error = %v", err)
	}
	if err := store.Set("b", []byte("2"), 0); err != nil {
		t.Fatalf("Set(b) error = %v", err)
	}
	if _, exists := store.Get("a"); !exists {
		t.Fatal("Get(a) did not find a value")
	}

	if err := store.Set("c", []byte("3"), 0); err != nil {
		t.Fatalf("Set(c) error = %v", err)
	}
	if _, exists := store.Get("b"); exists {
		t.Fatal("SET c did not evict least recently used key b")
	}
	requireStoreValue(t, store, "a", "1")
	requireStoreValue(t, store, "c", "3")
	requireStoreUsage(t, store, 4)

	if _, exists := store.lruCache.Cache["b"]; exists {
		t.Fatal("evicted key b remained in the LRU index")
	}
}

func TestStoreMemoryLimitRejectsNewMutationKeys(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Store) error
	}{
		{
			name: "append",
			mutate: func(store *Store) error {
				_, err := store.Append("a", []byte("b"))
				return err
			},
		},
		{
			name: "incr",
			mutate: func(store *Store) error {
				_, err := store.Incr("a")
				return err
			},
		},
		{
			name: "decr",
			mutate: func(store *Store) error {
				_, err := store.Decr("a")
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newLimitedStore(1)

			if err := test.mutate(store); !errors.Is(err, ErrMemoryLimitSurpassed) {
				t.Fatalf("mutation error = %v, want %v", err, ErrMemoryLimitSurpassed)
			}
			if _, exists := store.Get("a"); exists {
				t.Fatal("rejected mutation created a key")
			}
			requireStoreUsage(t, store, 0)
		})
	}
}

func TestStoreCleanupReleasesMemoryLimitUsage(t *testing.T) {
	t.Run("delete", func(t *testing.T) {
		store := newLimitedStore(2)
		if err := store.Set("a", []byte("b"), 0); err != nil {
			t.Fatalf("Set() error = %v", err)
		}
		store.Del([]string{"a"})
		requireStoreUsage(t, store, 0)
	})

	t.Run("passive expiry", func(t *testing.T) {
		store := newLimitedStore(2)
		expireStoreKey(t, store, "a", "b")
		store.Get("a")
		requireStoreUsage(t, store, 0)
	})

	t.Run("active expiry", func(t *testing.T) {
		store := newLimitedStore(2)
		expireStoreKey(t, store, "a", "b")
		store.activeExpireCycle(time.Millisecond)
		requireStoreUsage(t, store, 0)
	})
}

func TestStoreNewMutationKeysJoinAndUseLRU(t *testing.T) {
	tests := []struct {
		name     string
		maxBytes int64
		mutate   func(*Store) error
	}{
		{
			name:     "append",
			maxBytes: 4,
			mutate: func(store *Store) error {
				_, err := store.Append("a", []byte("1"))
				return err
			},
		},
		{
			name:     "incr",
			maxBytes: 4,
			mutate: func(store *Store) error {
				_, err := store.Incr("a")
				return err
			},
		},
		{
			name:     "decr",
			maxBytes: 5,
			mutate: func(store *Store) error {
				_, err := store.Decr("a")
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newLimitedStore(test.maxBytes)
			if err := test.mutate(store); err != nil {
				t.Fatalf("mutation error = %v", err)
			}
			requireLRUKey(t, store, "a", true)

			if err := store.Set("b", []byte("2"), 0); err != nil {
				t.Fatalf("Set(b) error = %v", err)
			}
			if _, exists := store.Get("a"); !exists {
				t.Fatal("Get(a) did not find a value")
			}
			if err := store.Set("c", []byte("3"), 0); err != nil {
				t.Fatalf("Set(c) error = %v", err)
			}

			if _, exists := store.Get("b"); exists {
				t.Fatal("recently used a did not outlive b")
			}
		})
	}
}

func TestStoreExistingMutationsRefreshLRU(t *testing.T) {
	tests := []struct {
		name     string
		maxBytes int64
		mutate   func(*Store) error
	}{
		{
			name:     "append",
			maxBytes: 5,
			mutate: func(store *Store) error {
				_, err := store.Append("a", []byte("x"))
				return err
			},
		},
		{
			name:     "incr",
			maxBytes: 4,
			mutate: func(store *Store) error {
				_, err := store.Incr("a")
				return err
			},
		},
		{
			name:     "decr",
			maxBytes: 5,
			mutate: func(store *Store) error {
				_, err := store.Decr("a")
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newLimitedStore(test.maxBytes)
			if err := store.Set("a", []byte("0"), 0); err != nil {
				t.Fatalf("Set(a) error = %v", err)
			}
			if err := store.Set("b", []byte("2"), 0); err != nil {
				t.Fatalf("Set(b) error = %v", err)
			}
			if err := test.mutate(store); err != nil {
				t.Fatalf("mutation error = %v", err)
			}
			if err := store.Set("c", []byte("3"), 0); err != nil {
				t.Fatalf("Set(c) error = %v", err)
			}

			if _, exists := store.Get("b"); exists {
				t.Fatal("mutated a did not outlive b")
			}
		})
	}
}

func TestStoreRemovalPathsClearLRU(t *testing.T) {
	t.Run("delete", func(t *testing.T) {
		store := NewStore()
		if err := store.Set("a", []byte("1"), 0); err != nil {
			t.Fatalf("Set(a) error = %v", err)
		}

		store.Del([]string{"a"})
		requireLRUKey(t, store, "a", false)
	})

	t.Run("passive expiry", func(t *testing.T) {
		store := NewStore()
		expireStoreKey(t, store, "a", "1")

		store.Get("a")
		requireLRUKey(t, store, "a", false)
	})

	t.Run("active expiry", func(t *testing.T) {
		store := NewStore()
		expireStoreKey(t, store, "a", "1")

		store.activeExpireCycle(time.Millisecond)
		requireLRUKey(t, store, "a", false)
	})
}

func TestStoreCapacityMutationsPurgeExpiredKeysBeforeEviction(t *testing.T) {
	tests := []struct {
		name       string
		maxBytes   int64
		setup      func(*Store) error
		mutate     func(*Store) error
		wantTarget string
	}{
		{
			name:     "set new key",
			maxBytes: 4,
			mutate: func(store *Store) error {
				return store.Set("c", []byte("3"), 0)
			},
			wantTarget: "3",
		},
		{
			name:     "append new key",
			maxBytes: 4,
			mutate: func(store *Store) error {
				_, err := store.Append("c", []byte("3"))
				return err
			},
			wantTarget: "3",
		},
		{
			name:     "incr new key",
			maxBytes: 4,
			mutate: func(store *Store) error {
				_, err := store.Incr("c")
				return err
			},
			wantTarget: "1",
		},
		{
			name:     "decr new key",
			maxBytes: 5,
			mutate: func(store *Store) error {
				_, err := store.Decr("c")
				return err
			},
			wantTarget: "-1",
		},
		{
			name:     "set existing key",
			maxBytes: 6,
			setup: func(store *Store) error {
				return store.Set("c", []byte("0"), 0)
			},
			mutate: func(store *Store) error {
				return store.Set("c", []byte("xx"), 0)
			},
			wantTarget: "xx",
		},
		{
			name:     "append existing key",
			maxBytes: 6,
			setup: func(store *Store) error {
				return store.Set("c", []byte("0"), 0)
			},
			mutate: func(store *Store) error {
				_, err := store.Append("c", []byte("x"))
				return err
			},
			wantTarget: "0x",
		},
		{
			name:     "incr existing key",
			maxBytes: 6,
			setup: func(store *Store) error {
				return store.Set("c", []byte("9"), 0)
			},
			mutate: func(store *Store) error {
				_, err := store.Incr("c")
				return err
			},
			wantTarget: "10",
		},
		{
			name:     "decr existing key",
			maxBytes: 6,
			setup: func(store *Store) error {
				return store.Set("c", []byte("0"), 0)
			},
			mutate: func(store *Store) error {
				_, err := store.Decr("c")
				return err
			},
			wantTarget: "-1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newLimitedStore(test.maxBytes)
			if err := store.Set("a", []byte("1"), time.Hour); err != nil {
				t.Fatalf("Set(a) error = %v", err)
			}
			if err := store.Set("b", []byte("2"), 0); err != nil {
				t.Fatalf("Set(b) error = %v", err)
			}
			if test.setup != nil {
				if err := test.setup(store); err != nil {
					t.Fatalf("setup error = %v", err)
				}
			}
			if _, exists := store.Get("a"); !exists {
				t.Fatal("Get(a) did not find a value")
			}

			store.mu.Lock()
			store.expire.expireData["a"] = time.Now().Add(-time.Millisecond).UnixMilli()
			store.mu.Unlock()

			if err := test.mutate(store); err != nil {
				t.Fatalf("mutation error = %v", err)
			}

			if _, exists := store.Get("a"); exists {
				t.Fatal("expired key a remained in the Store")
			}
			requireStoreValue(t, store, "b", "2")
			requireStoreValue(t, store, "c", test.wantTarget)
			requireLRUKey(t, store, "a", false)
		})
	}
}

func TestStoreStats(t *testing.T) {
	t.Run("get hit miss and passive expiry", func(t *testing.T) {
		store := NewStore()
		store.Get("missing")
		if err := store.Set("a", []byte("1"), 0); err != nil {
			t.Fatalf("Set(a) error = %v", err)
		}
		store.Get("a")
		expireStoreKey(t, store, "b", "1")
		store.Get("b")

		requireStats(t, store, Stats{Items: 1, Bytes: 2, Hits: 1, Misses: 2, Expired: 1})
	})

	t.Run("active expiry", func(t *testing.T) {
		store := NewStore()
		expireStoreKey(t, store, "a", "1")
		store.activeExpireCycle(time.Millisecond)

		requireStats(t, store, Stats{Expired: 1})
	})

	t.Run("pressure expiry", func(t *testing.T) {
		store := newLimitedStore(4)
		if err := store.Set("a", []byte("1"), time.Hour); err != nil {
			t.Fatalf("Set(a) error = %v", err)
		}
		if err := store.Set("b", []byte("2"), 0); err != nil {
			t.Fatalf("Set(b) error = %v", err)
		}
		store.Get("a")
		store.mu.Lock()
		store.expire.expireData["a"] = time.Now().Add(-time.Millisecond).UnixMilli()
		store.mu.Unlock()
		if err := store.Set("c", []byte("3"), 0); err != nil {
			t.Fatalf("Set(c) error = %v", err)
		}

		requireStats(t, store, Stats{Items: 2, Bytes: 4, Hits: 1, Expired: 1})
	})

	t.Run("LRU eviction", func(t *testing.T) {
		store := newLimitedStore(4)
		if err := store.Set("a", []byte("1"), 0); err != nil {
			t.Fatalf("Set(a) error = %v", err)
		}
		if err := store.Set("b", []byte("2"), 0); err != nil {
			t.Fatalf("Set(b) error = %v", err)
		}
		if err := store.Set("c", []byte("3"), 0); err != nil {
			t.Fatalf("Set(c) error = %v", err)
		}

		requireStats(t, store, Stats{Items: 2, Bytes: 4, Evicted: 1})
	})

	t.Run("rejected write", func(t *testing.T) {
		store := newLimitedStore(1)
		if err := store.Set("a", []byte("12"), 0); !errors.Is(err, ErrMemoryLimitSurpassed) {
			t.Fatalf("Set() error = %v, want %v", err, ErrMemoryLimitSurpassed)
		}

		requireStats(t, store, Stats{Rejected: 1})
	})
}
