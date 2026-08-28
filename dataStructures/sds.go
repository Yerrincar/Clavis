package ds

import (
	"errors"
)

type SDS struct {
	buf []byte
}

const MaxStringSize = 512 << 20
const oneMB int = 1 << 20

var ErrNegativeSize = errors.New("negative size string")
var ErrStringTooLarge = errors.New("string exceeds maximum allowed size")

func NewSDS(data []byte) (*SDS, error) {
	if len(data) == 0 {
		return &SDS{}, nil
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

/*
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
		newCap, err := s.nextCapacity(addedLen, len(s.buf))
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
*/

func (s *SDS) sdsIncrLen(incr int) error {
	if s.CheckStringLength(incr, len(s.buf)) {
		return ErrStringTooLarge
	}
	newLen := len(s.buf) + incr

	if incr >= 0 {
		if newLen > cap(s.buf) {
			return errors.New("increment exceeds allocated capacity")
		}
	} else {
		if newLen < 0 {
			return errors.New("decrement results in negative length")
		}
	}
	s.buf = s.buf[:newLen]

	return nil
}

func (s *SDS) MakeRoomFor(additionalBytes int) error {
	if additionalBytes == 0 {
		return nil
	}

	if additionalBytes < 0 {
		return ErrNegativeSize
	}

	if s.CheckStringLength(additionalBytes, len(s.buf)) {
		return ErrStringTooLarge
	}

	available := s.Free()

	if available >= additionalBytes {
		return nil
	}

	newCap := s.nextCapacity(additionalBytes, len(s.buf))

	newBuf := make([]byte, len(s.buf), newCap)
	copy(newBuf, s.buf)
	s.buf = newBuf
	return nil
}

func (s *SDS) nextCapacity(added, actual int) int {
	required := added + actual
	doubleLen := required * 2

	if required < oneMB {
		return doubleLen
	}
	return required + oneMB
}

func (s *SDS) CheckStringLength(data, size int) bool {
	return data > MaxStringSize-size
}
