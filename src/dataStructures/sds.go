package sds

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

func (s *SDS) Available() int {
	return cap(s.buf) - len(s.buf)
}

func (s *SDS) AvailableWritableRegion(maxCap int) ([]byte, error) {
	if maxCap <= s.Available() && maxCap >= 0 {
		return s.buf[len(s.buf) : len(s.buf)+maxCap], nil
	}
	return nil, errors.New("max cap exceeds available writable bytes")
}

func (s *SDS) Bytes() []byte {
	return s.buf
}

func (s *SDS) RemoveFreeSpace() {
	s.Resize(s.Len())
}

func (s *SDS) Resize(size int) {
	if size < 0 {
		return
	}

	if cap(s.buf) == size {
		return
	}

	if size < len(s.buf) {
		s.buf = s.buf[:size]
	}

	newCap := size
	newBuf := make([]byte, len(s.buf), newCap)
	copy(newBuf, s.buf)
	s.buf = newBuf
}

func (s *SDS) SubStr(start, size int) {
	oldLen := len(s.buf)
	if start < 0 || size < 0 {
		return
	}

	if start >= oldLen {
		start = 0
		size = 0
	}

	if size > oldLen-start {
		size = oldLen - start
	}

	copy(s.buf, s.buf[start:start+size])
	s.buf = s.buf[:size]
}

func (s *SDS) IncrLen(incr int) error {
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

	available := s.Available()

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
