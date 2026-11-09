package ds

import "testing"

func TestLRUCacheMaintainsRecencyOrder(t *testing.T) {
	cache := NewLRUCache()
	cache.Put("a")
	cache.Put("b")
	cache.Put("c")

	a := cache.Cache["a"]
	cache.Put("a")
	if cache.head != a || cache.Tail.Key != "b" {
		t.Fatalf("Put(existing) order = head %q, tail %q, want a, b", cache.head.Key, cache.Tail.Key)
	}

	if key, found := cache.Evict(""); !found || key != "b" {
		t.Fatalf("Evict() = (%q, %t), want (b, true)", key, found)
	}
	if cache.head.Key != "a" || cache.Tail.Key != "c" {
		t.Fatalf("Evict() order = head %q, tail %q, want a, c", cache.head.Key, cache.Tail.Key)
	}

	if key, found := cache.Evict("c"); !found || key != "a" {
		t.Fatalf("Evict(c) = (%q, %t), want (a, true)", key, found)
	}
	if cache.head != cache.Tail || cache.head.Key != "c" {
		t.Fatal("Evict(c) did not retain c as the only node")
	}

	if key, found := cache.Evict("c"); found || key != "" {
		t.Fatalf("Evict(c) with only c = (%q, %t), want (empty, false)", key, found)
	}
}
