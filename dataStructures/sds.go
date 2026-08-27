package ds

import (
	"errors"
)

type SDS struct {
	buf []byte
}

const MaxStringSize = 512 << 20
const oneMB int = 1 << 20

var ErrEmptyData = errors.New("empty string")
var ErrStringTooLarge = errors.New("string exceeds maximum allowed size")

func NewSDS(data []byte) (*SDS, error) {
	if len(data) == 0 {
		return nil, nil
	}
	if len(data) > MaxStringSize {
		return nil, ErrStringTooLarge
	}
	b := make([]byte, len(data))
	copy(b, data)

	return &SDS{
		buf: b,
	}, nil
}

func (s *SDS) Len() int {
	return len(s.buf)
}

func (s *SDS) Cap() int {
	return cap(s.buf)
}

func (s *SDS) Free() int {
	return cap(s.buf) - len(s.buf)
}

func (s *SDS) Bytes() []byte {
	return s.buf
}

func (s *SDS) Append(data []byte) error {
	addedLen := len(data)
	if addedLen == 0 {
		return nil
	}

	if s.CheckStringLength(len(data), len(s.buf)) {
		return ErrStringTooLarge
	}

	oldLen := len(s.buf)
	required := oldLen + addedLen
	available := s.Free()

	if available < addedLen {
		newCap, err := s.nextCapacity(data, len(s.buf))
		if err != nil {
			return err
		}
		newBuf := make([]byte, len(s.buf), newCap)
		copy(newBuf, s.buf)

		s.buf = newBuf
	}
	s.buf = s.buf[:required]
	copy(s.buf[oldLen:], data)
	return nil
}

func (s *SDS) nextCapacity(added []byte, actual int) (int, error) {
	addedLen := len(added)
	required := addedLen + actual
	doubleLen := required * 2

	if required < oneMB {
		return doubleLen, nil
	}
	return required + oneMB, nil
}

func (s *SDS) CheckStringLength(data, size int) bool {
	return data > MaxStringSize-size
}
