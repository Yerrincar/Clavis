package store

import (
	sds "Clavis/src/dataStructures"
	"errors"
	"sync"
)

const MaxStringSize = 512 << 20

var ErrStringTooLarge = errors.New("string exceeds maximum allowed size")

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
