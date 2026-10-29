package store

import (
	sds "Clavis/src/dataStructures"
	"errors"
	"math"
	"strconv"
	"sync"
)

const MaxStringSize = 512 << 20

var ErrStringTooLarge = errors.New("string exceeds maximum allowed size")
var ErrInvalidInteger = errors.New("value is not an integer or out of range")

type Store struct {
	mu   sync.RWMutex
	data map[string]*sds.SDS
}

func NewStore() *Store {
	return &Store{
		data: make(map[string]*sds.SDS),
	}
}

func (s *Store) Set(key string, value *sds.SDS) error {

	if value.Len() > MaxStringSize {
		return ErrStringTooLarge
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.data[key] = value

	return nil
}

func (s *Store) Get(key string) (*sds.SDS, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, exists := s.data[key]
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
		if _, exists := s.data[key]; !exists {
			continue
		}
		delete(s.data, key)
		keysDeleted++
	}

	return keysDeleted
}

func (s *Store) Exists(keys []string) int {

	s.mu.RLock()
	defer s.mu.RUnlock()

	existingKeys := 0

	for _, key := range keys {
		if _, exists := s.data[key]; exists {
			existingKeys++
		}
	}

	return existingKeys
}

func (s *Store) Append(key string, suffix []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, exists := s.data[key]

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

	if _, exists := s.data[key]; !exists {
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

	if _, exists := s.data[key]; !exists {
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
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, exists := s.data[key]; !exists {
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
