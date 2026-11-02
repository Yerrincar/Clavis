package store

import (
	"errors"
	"sync"
	"testing"
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

func TestStoreBasicStringCommands(t *testing.T) {
	store := NewStore()
	input := []byte("hello")

	if err := store.Set("key", input); err != nil {
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

	if err := store.Set("invalid", []byte("not-an-integer")); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if _, err := store.Incr("invalid"); !errors.Is(err, ErrInvalidInteger) {
		t.Fatalf("Incr(invalid) error = %v, want %v", err, ErrInvalidInteger)
	}
}

func TestStoreReadersRemainSafeDuringUpdates(t *testing.T) {
	store := NewStore()
	if err := store.Set("text", []byte("a")); err != nil {
		t.Fatalf("Set(text) error = %v", err)
	}
	if err := store.Set("counter", []byte("0")); err != nil {
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
