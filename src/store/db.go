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

func (s *Store) Append(key string, actualValue, value *sds.SDS) error {

	if actualValue.Len()+value.Len() > MaxStringSize {
		return ErrStringTooLarge
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.data[key] = value

	return nil
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

func (s *Store) Ping(message []byte) ([]byte, error) {
	if message == nil {
		return []byte("PONG"), nil
	}
	return message, nil
}
