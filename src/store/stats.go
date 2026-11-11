package store

import "sync/atomic"

type Stats struct {
	Items    int64
	Bytes    int64
	Hits     uint64
	Misses   uint64
	Expired  uint64
	Evicted  uint64
	Rejected uint64
}

type counters struct {
	hits     atomic.Uint64
	misses   atomic.Uint64
	expired  atomic.Uint64
	evicted  atomic.Uint64
	rejected atomic.Uint64
}
