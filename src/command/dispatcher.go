package command

import (
	ds "Clavis/src/dataStructures"
	"Clavis/src/store"
	"bytes"
	"errors"
	"fmt"
	"time"
)

type ReplyType int

const (
	TypeStatus ReplyType = iota
	TypeString
	TypeError
	TypeInt
	TypeBulk
	TypeNull
	TypeArray
)

type Reply struct {
	Type   ReplyType
	Bytes  *ds.SDS
	String string
	Num    int64
	Array  []*Reply
	Err    error
}

var (
	EmptyArgs = errors.New("empty command")
	NullArgs  = errors.New("null command arguments are not supported")
)

type Service struct {
	Store *store.Store
}

func NewService() *Service {
	return &Service{
		Store: store.NewStore(),
	}
}

func (s *Service) Execute(args []*ds.SDS) *Reply {
	if len(args) == 0 {
		return &Reply{Type: TypeError, Err: EmptyArgs}
	}

	for _, arg := range args {
		if arg == nil {
			return &Reply{Type: TypeError, Err: NullArgs}
		}
	}

	cmd := args[0].BorrowBytes()

	switch {
	case bytes.Equal(cmd, []byte("SET")):
		if len(args) < 3 || len(args) > 4 {
			return &Reply{Type: TypeError, Err: errors.New("wrong number of arguments for 'set' command")}
		}

		key := string(args[1].BorrowBytes())
		if key == "" {
			return &Reply{Type: TypeError, Err: errors.New("no key found for that argument")}
		}
		value := args[2]

		var ttl time.Duration
		if len(args) == 4 {
			ttlValue, err := time.ParseDuration(string(args[3].BorrowBytes()))
			if err != nil {
				return &Reply{Type: TypeError, Err: err}
			}
			ttl = ttlValue
		}

		err := s.Store.Set(key, value.BorrowBytes(), ttl)
		if err != nil {
			return &Reply{Type: TypeError, Err: err}
		}

		return &Reply{Type: TypeStatus, String: "OK"}

	case bytes.Equal(cmd, []byte("GET")):
		if len(args) != 2 {
			return &Reply{Type: TypeError, Err: errors.New("wrong number of arguments for 'get' command")}
		}

		key := string(args[1].BorrowBytes())
		if key == "" {
			return &Reply{Type: TypeError, Err: errors.New("no key found for that argument")}
		}
		result, exist := s.Store.Get(key)
		if !exist {
			return &Reply{Type: TypeNull, Bytes: nil}
		}

		return &Reply{Type: TypeBulk, Bytes: result}

	case bytes.Equal(cmd, []byte("DEL")):
		if len(args) == 1 {
			return &Reply{Type: TypeError, Err: errors.New("wrong number of arguments for 'del' command")}
		}
		keys := make([]string, 0)

		for _, key := range args[1:] {
			keys = append(keys, string(key.BorrowBytes()))
		}
		deletedKeys := s.Store.Del(keys)
		return &Reply{Type: TypeInt, Num: int64(deletedKeys)}

	case bytes.Equal(cmd, []byte("EXISTS")):
		if len(args) == 1 {
			return &Reply{Type: TypeError, Err: errors.New("wrong number of arguments for 'exists' command")}
		}
		keys := make([]string, 0)

		for _, key := range args[1:] {
			keys = append(keys, string(key.BorrowBytes()))
		}
		existingKeys := s.Store.Exists(keys)
		return &Reply{Type: TypeInt, Num: int64(existingKeys)}

	case bytes.Equal(cmd, []byte("STRLEN")):
		if len(args) != 2 {
			return &Reply{Type: TypeError, Err: errors.New("wrong number of arguments for 'strlen' command")}
		}

		key := string(args[1].BorrowBytes())

		length := s.Store.Strlen(key)
		return &Reply{Type: TypeInt, Num: int64(length)}

	case bytes.Equal(cmd, []byte("APPEND")):
		if len(args) != 3 {
			return &Reply{Type: TypeError, Err: errors.New("wrong number of arguments for 'append' command")}
		}

		key := string(args[1].BorrowBytes())

		added, err := s.Store.Append(key, args[2].BorrowBytes())
		if err != nil {
			return &Reply{Type: TypeError, Err: err}
		}

		return &Reply{Type: TypeInt, Num: int64(added)}

	case bytes.Equal(cmd, []byte("INCR")):
		if len(args) != 2 {
			return &Reply{Type: TypeError, Err: errors.New("wrong number of arguments for 'incr' command")}
		}

		key := string(args[1].BorrowBytes())

		value, err := s.Store.Incr(key)
		if err != nil {
			return &Reply{Type: TypeError, Err: err}
		}

		return &Reply{Type: TypeInt, Num: value}

	case bytes.Equal(cmd, []byte("DECR")):
		if len(args) != 2 {
			return &Reply{Type: TypeError, Err: errors.New("wrong number of arguments for 'decr' command")}
		}

		key := string(args[1].BorrowBytes())

		value, err := s.Store.Decr(key)
		if err != nil {
			return &Reply{Type: TypeError, Err: err}
		}

		return &Reply{Type: TypeInt, Num: value}

	case bytes.Equal(cmd, []byte("PING")):
		switch len(args) {
		case 1:
			return &Reply{Type: TypeStatus, String: "PONG"}
		case 2:
			return &Reply{Type: TypeBulk, Bytes: args[1]}
		default:
			return &Reply{Type: TypeError, Err: errors.New("wrong number of arguments for 'ping' command")}
		}

	case bytes.Equal(cmd, []byte("STATS")):
		if len(args) != 1 {
			return &Reply{Type: TypeError, Err: errors.New("wrong number of arguments for 'stats' command")}
		}

		stats := s.Store.StatsInfo()

		response := fmt.Sprintf("Items: %d\nBytes: %d\nHits: %d\nMisses: %d\nExpired: %d\nEvicted: %d\nRejected: %d", stats.Items, stats.Bytes, stats.Hits, stats.Misses, stats.Expired, stats.Evicted, stats.Rejected)
		responseSDS, err := ds.NewSDSFromBytes([]byte(response))
		if err != nil {
			return &Reply{Type: TypeError, Err: err}
		}
		return &Reply{Type: TypeBulk, Bytes: responseSDS}

	default:
		return &Reply{Type: TypeError,
			Err: errors.New("No commands found in the message. SET or GET are the only commands supported for now")}
	}
}
