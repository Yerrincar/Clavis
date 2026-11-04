package store

import (
	"time"
)

type Expire struct {
	expireData map[string]int64
}

func NewExpire() *Expire {
	return &Expire{
		expireData: make(map[string]int64),
	}
}

func (e *Expire) createKeyExpirationTime(key string, ttl time.Duration) {
	expirationTime := time.Now().Add(ttl).UnixMilli()

	e.expireData[key] = expirationTime
}

func (e *Expire) alreadyExpiredKey(key string) bool {
	ttlValue, exists := e.expireData[key]
	if !exists {
		return false
	}
	return e.alreadyExpiredAt(ttlValue)
}

func (e *Expire) alreadyExpiredAt(ttlValue int64) bool {
	return time.Now().UnixMilli() >= ttlValue
}
