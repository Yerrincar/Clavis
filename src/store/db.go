package store

import (
	ds "Clavis/src/dataStructures"
	"context"
	"errors"
	"math"
	"strconv"
	"sync"
	"time"
)

const MaxStringSize = 512 << 20

var ErrStringTooLarge = errors.New("string exceeds maximum allowed size")
var ErrMemoryLimitSurpassed = errors.New("max cache limit exceeded with that addition")
var ErrInvalidInteger = errors.New("value is not an integer or out of range")

type Store struct {
	mu          sync.RWMutex
	counters    counters
	actualUsage int64
	limit       *Limit
	data        map[string]*ds.SDS
	lruCache    *ds.LRUCache
	expire      *Expire
}

func NewStore() *Store {
	return &Store{
		limit:    NewLimit(),
		data:     make(map[string]*ds.SDS),
		lruCache: ds.NewLRUCache(),
		expire:   NewExpire(),
	}
}

func (s *Store) Set(key string, value []byte, ttl time.Duration) error {

	s.mu.Lock()
	defer s.mu.Unlock()

	newCost := len(key) + len(value)
	newSDS, err := ds.NewSDSFromBytes(value)
	if err != nil {
		return err
	}

	if ttl < 0 {
		return errors.New("invalid ttl")
	}

	current, exists := s.getLockedKey(key)
	oldCost := 0
	if exists {
		oldCost = len(key) + current.Len()
	}
	if err := s.makeRoomLocked(key, oldCost, newCost); err != nil {
		return err
	}

	if ttl == 0 {
		delete(s.expire.expireData, key)
	} else {
		s.expire.createKeyExpirationTime(key, ttl)
	}
	s.data[key] = newSDS
	s.actualUsage = s.actualUsage - int64(oldCost) + int64(newCost)
	s.lruCache.Put(key)

	return nil
}

func (s *Store) getLockedKey(key string) (*ds.SDS, bool) {
	value, exists := s.data[key]
	if !exists {
		return nil, false
	}

	if s.expire.alreadyExpiredKey(key) {
		s.actualUsage -= int64(len(key) + value.Len())
		delete(s.data, key)
		delete(s.expire.expireData, key)
		s.lruCache.Remove(key)
		s.counters.expired.Add(1)
		return nil, false
	}

	return value, true

}

func (s *Store) Get(key string) (*ds.SDS, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	value, exists := s.getLockedKey(key)
	if !exists {
		s.counters.misses.Add(1)
		return nil, false
	}

	node := s.lruCache.Cache[key]

	s.lruCache.MoveToFront(node)
	s.counters.hits.Add(1)
	return value, true
}

func (s *Store) Del(keys []string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	keysDeleted := 0

	for _, key := range keys {
		value, exists := s.getLockedKey(key)
		if !exists {
			continue
		}
		s.actualUsage -= int64(len(key) + value.Len())
		delete(s.data, key)
		delete(s.expire.expireData, key)
		s.lruCache.Remove(key)
		keysDeleted++
	}

	return keysDeleted
}

func (s *Store) Exists(keys []string) int {

	s.mu.Lock()
	defer s.mu.Unlock()

	existingKeys := 0

	for _, key := range keys {
		_, exists := s.getLockedKey(key)
		if exists {
			node := s.lruCache.Cache[key]
			s.lruCache.MoveToFront(node)
			existingKeys++
		}
	}

	return existingKeys
}

func (s *Store) Append(key string, suffix []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, exists := s.getLockedKey(key)
	oldCost := 0
	newCost := len(key) + len(suffix)
	var replacement *ds.SDS

	if !exists {
		var err error
		replacement, err = ds.NewSDSFromBytes(suffix)
		if err != nil {
			return 0, err
		}
	} else {
		oldCost = len(key) + current.Len()
		newCost = len(key) + current.Len() + len(suffix)
		var err error
		replacement, err = ds.NewSDS(current.Len() + len(suffix))
		if err != nil {
			return 0, err
		}
		if err := replacement.Append(current.BorrowBytes()); err != nil {
			return 0, err
		}
		if err := replacement.Append(suffix); err != nil {
			return 0, err
		}
	}

	if err := s.makeRoomLocked(key, oldCost, newCost); err != nil {
		return 0, err
	}

	s.data[key] = replacement
	s.lruCache.Put(key)
	s.actualUsage = s.actualUsage - int64(oldCost) + int64(newCost)

	return replacement.Len(), nil
}

func (s *Store) Decr(key string) (int64, error) {

	s.mu.Lock()
	defer s.mu.Unlock()

	value, exists := s.getLockedKey(key)
	oldCost := 0
	number := int64(-1)
	var replacement *ds.SDS
	var err error
	if !exists {
		replacement, err = ds.NewSDSFromBytes([]byte("-1"))
		if err != nil {
			return 0, err
		}
	} else {
		number, err = parseDecimalInt64(value.BorrowBytes())
		if err != nil {
			return -1, err
		}
		if number == math.MinInt64 {
			return -1, ErrInvalidInteger
		}
		number--

		var buffer [20]byte
		encoded := strconv.AppendInt(buffer[:0], number, 10)
		replacement, err = ds.NewSDSFromBytes(encoded)
		if err != nil {
			return -1, err
		}
		oldCost = len(key) + value.Len()
	}
	newCost := len(key) + replacement.Len()
	if err := s.makeRoomLocked(key, oldCost, newCost); err != nil {
		return -1, err
	}

	s.data[key] = replacement
	s.lruCache.Put(key)
	s.actualUsage = s.actualUsage - int64(oldCost) + int64(newCost)

	return number, nil
}

func (s *Store) Incr(key string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	value, exists := s.getLockedKey(key)
	oldCost := 0
	number := int64(1)
	var replacement *ds.SDS
	var err error
	if !exists {
		replacement, err = ds.NewSDSFromBytes([]byte("1"))
		if err != nil {
			return -1, err
		}
	} else {
		number, err = parseDecimalInt64(value.BorrowBytes())
		if err != nil {
			return -1, err
		}
		if number == math.MaxInt64 {
			return -1, ErrInvalidInteger
		}
		number++

		var buffer [20]byte
		encoded := strconv.AppendInt(buffer[:0], number, 10)
		replacement, err = ds.NewSDSFromBytes(encoded)
		if err != nil {
			return -1, err
		}
		oldCost = len(key) + value.Len()
	}
	newCost := len(key) + replacement.Len()
	if err := s.makeRoomLocked(key, oldCost, newCost); err != nil {
		return -1, err
	}

	s.data[key] = replacement
	s.lruCache.Put(key)
	s.actualUsage = s.actualUsage - int64(oldCost) + int64(newCost)

	return number, nil
}

func (s *Store) Strlen(key string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.getLockedKey(key); !exists {
		return 0
	}

	value := s.data[key]
	node := s.lruCache.Cache[key]
	s.lruCache.MoveToFront(node)
	return value.Len()
}

func parseDecimalInt64(data []byte) (int64, error) {
	if len(data) == 0 {
		return 0, ErrInvalidInteger
	}

	if len(data) == 1 && data[0] == '0' {
		return 0, nil
	}

	negative := data[0] == '-'
	if negative {
		data = data[1:]
	}

	if len(data) == 0 || data[0] < '1' || data[0] > '9' {
		return 0, ErrInvalidInteger
	}

	limit := uint64(math.MaxInt64)
	if negative {
		limit++
	}

	var value uint64
	for _, b := range data {
		if b < '0' || b > '9' {
			return 0, ErrInvalidInteger
		}

		digit := uint64(b - '0')
		if value > limit/10 || (value == limit/10 && digit > limit%10) {
			return 0, ErrInvalidInteger
		}

		value = value*10 + digit
	}

	if negative {
		if value == uint64(math.MaxInt64)+1 {
			return math.MinInt64, nil
		}
		return -int64(value), nil
	}

	return int64(value), nil
}

func (s *Store) StartActiveExpireCycle(ctx context.Context, intervalo time.Duration) {
	ticker := time.NewTicker(intervalo)

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.activeExpireCycle(1 * time.Millisecond)
			}
		}
	}()
}

func (s *Store) activeExpireCycle(timeBudget time.Duration) {
	startTime := time.Now()

	for {
		if time.Since(startTime) >= timeBudget {
			break
		}

		s.mu.Lock()

		totalExpires := len(s.expire.expireData)
		if totalExpires == 0 {
			s.mu.Unlock()
			break
		}

		sampled := 0
		expired := 0
		maxSamples := 20
		if totalExpires < maxSamples {
			maxSamples = totalExpires
		}

		for key, when := range s.expire.expireData {
			if sampled >= maxSamples {
				break
			}
			sampled++

			if s.expire.alreadyExpiredAt(when) {
				value, exists := s.data[key]
				if exists {
					s.actualUsage -= int64(len(key) + value.Len())
					delete(s.data, key)
					s.lruCache.Remove(key)
					s.counters.expired.Add(1)
				}
				delete(s.expire.expireData, key)
				expired++
			}
		}

		s.mu.Unlock()

		if sampled == 0 {
			break
		}

		if expired*4 > sampled {
			continue
		}

		break
	}
}

func (s *Store) makeRoomLocked(key string, oldCost, newCost int) error {
	if int64(newCost) > s.limit.MaxBytes {
		s.counters.rejected.Add(1)
		return ErrMemoryLimitSurpassed
	}

	if s.actualUsage-int64(oldCost)+int64(newCost) > s.limit.MaxBytes {
		s.removeExpiredLockedKeys(key)
	}

	for s.actualUsage-int64(oldCost)+int64(newCost) > s.limit.MaxBytes {
		victimKey, found := s.lruCache.Evict(key)
		if !found {
			s.counters.rejected.Add(1)
			return ErrMemoryLimitSurpassed
		}

		value, exists := s.getLockedKey(victimKey)
		if !exists {
			continue
		}
		s.actualUsage -= int64(len(victimKey) + value.Len())
		delete(s.data, victimKey)
		delete(s.expire.expireData, victimKey)
		s.counters.evicted.Add(1)
	}

	return nil
}

func (s *Store) removeExpiredLockedKeys(excludedKey string) {
	for key, expirationTime := range s.expire.expireData {
		if key == excludedKey {
			continue
		}
		if !s.expire.alreadyExpiredAt(expirationTime) {
			continue
		}
		value, exists := s.data[key]
		if exists {
			s.actualUsage -= int64(len(key) + value.Len())
			delete(s.data, key)
			s.lruCache.Remove(key)
			s.counters.expired.Add(1)
		}
		delete(s.expire.expireData, key)
	}
}

func (s *Store) StatsInfo() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return Stats{
		Items:    int64(len(s.data)),
		Bytes:    s.actualUsage,
		Hits:     s.counters.hits.Load(),
		Misses:   s.counters.misses.Load(),
		Expired:  s.counters.expired.Load(),
		Evicted:  s.counters.evicted.Load(),
		Rejected: s.counters.rejected.Load(),
	}
}
