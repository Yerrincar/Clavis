package command

import (
	sds "Clavis/src/dataStructures"
	"Clavis/src/store"
	"bytes"
	"errors"
)

type ReplyType int

const (
	TypeStatus ReplyType = iota
	TypeError
	TypeInt
	TypeBulk
	TypeNull
	TypeArray
)

type Reply struct {
	Type   ReplyType
	Bytes  *sds.SDS
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
	store *store.Store
}

func NewService() *Service {
	return &Service{
		store: store.NewStore(),
	}
}

func (s *Service) Execute(args []*sds.SDS) *Reply {
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
		if len(args) != 3 {
			return &Reply{Type: TypeError, Err: errors.New("wrong number of arguments for 'set' command")}
		}

		key := string(args[1].BorrowBytes())
		if key == "" {
			return &Reply{Type: TypeError, Err: errors.New("no key found for that argument")}
		}
		value := args[2]

		err := s.store.Set(key, value)
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
		result, exist := s.store.Get(key)
		if !exist {
			return &Reply{Type: TypeNull, Bytes: nil}
		}

		return &Reply{Type: TypeBulk, Bytes: result}

	default:
		return &Reply{Type: TypeError,
			Err: errors.New("No commands found in the message. SET or GET are the only commands supported for now")}
	}
}
