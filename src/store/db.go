package store

import (
	sds "Clavis/src/dataStructures"
	"context"
	"errors"
	"math"
	"strconv"
	"sync"
	"time"
)

const MaxStringSize = 512 << 20

var ErrStringTooLarge = errors.New("string exceeds maximum allowed size")
var ErrInvalidInteger = errors.New("value is not an integer or out of range")

type Store struct {
	mu     sync.RWMutex
	data   map[string]*sds.SDS
	expire *Expire
}

func NewStore() *Store {
	return &Store{
		data:   make(map[string]*sds.SDS),
		expire: NewExpire(),
	}
}

func (s *Store) Set(key string, value []byte, ttl time.Duration) error {

	if len(value) > MaxStringSize {
		return ErrStringTooLarge
	}

	newSDS, err := sds.NewSDSFromBytes(value)
	if err != nil {
		return err
	}

	if ttl < 0 {
		return errors.New("invalid ttl")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if ttl == 0 {
		delete(s.expire.expireData, key)
	} else {
		s.expire.createKeyExpirationTime(key, ttl)
	}

	s.data[key] = newSDS

	return nil
}

func (s *Store) getLockedKey(key string) (*sds.SDS, bool) {
	value, exists := s.data[key]
	if !exists {
		return nil, false
	}

	if s.expire.alreadyExpiredKey(key) {
		delete(s.data, key)
		delete(s.expire.expireData, key)
		return nil, false
	}

	return value, true

}

func (s *Store) Get(key string) (*sds.SDS, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	value, exists := s.getLockedKey(key)
	if !exists {
		return nil, false
	}

	return value, true
}

func (s *Store) Del(keys []string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	keysDeleted := 0

	for _, key := range keys {
		if _, exists := s.getLockedKey(key); !exists {
			continue
		}
		delete(s.data, key)
		delete(s.expire.expireData, key)
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
			existingKeys++
		}
	}

	return existingKeys
}

func (s *Store) Append(key string, suffix []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, exists := s.getLockedKey(key)

	if !exists {
		replacement, err := sds.NewSDSFromBytes(suffix)
		if err != nil {
			return 0, err
		}
		s.data[key] = replacement
		return replacement.Len(), nil

	}

	replacement, err := sds.NewSDS(current.Len() + len(suffix))
	if err != nil {
		return 0, err
	}

	//sust for the combination of the new one instead of adding to the existing one
	err = replacement.Append(current.BorrowBytes())
	if err != nil {
		return 0, err
	}

	err = replacement.Append(suffix)
	if err != nil {
		return 0, err
	}

	s.data[key] = replacement

	return replacement.Len(), nil
}

func (s *Store) Decr(key string) (int64, error) {

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.getLockedKey(key); !exists {
		value, err := sds.NewSDSFromBytes([]byte("-1"))
		if err != nil {
			return 0, err
		}

		s.data[key] = value
		return -1, nil
	}

	value := s.data[key]

	number, err := parseDecimalInt64(value.BorrowBytes())
	if err != nil {
		return -1, err
	}

	if number == math.MinInt64 {
		return -1, ErrInvalidInteger
	}
	number--

	var buffer [20]byte
	encoded := strconv.AppendInt(buffer[:0], number, 10)

	replacement, err := sds.NewSDSFromBytes(encoded)
	if err != nil {
		return -1, err
	}

	s.data[key] = replacement

	return number, nil
}

func (s *Store) Incr(key string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.getLockedKey(key); !exists {
		value, err := sds.NewSDSFromBytes([]byte("1"))
		if err != nil {
			return -1, err
		}

		s.data[key] = value
		return 1, nil
	}

	value := s.data[key]

	number, err := parseDecimalInt64(value.BorrowBytes())
	if err != nil {
		return -1, err
	}
	if number == math.MaxInt64 {
		return -1, ErrInvalidInteger
	}
	number++

	var buffer [20]byte
	encoded := strconv.AppendInt(buffer[:0], number, 10)

	replacement, err := sds.NewSDSFromBytes(encoded)
	if err != nil {
		return -1, err
	}

	s.data[key] = replacement

	return number, nil
}

func (s *Store) Strlen(key string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.getLockedKey(key); !exists {
		return 0
	}

	value := s.data[key]
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
				delete(s.expire.expireData, key)
				delete(s.data, key)
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
