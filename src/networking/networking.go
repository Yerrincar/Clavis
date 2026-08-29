package networking

import (
	"io"
)

func ReadConn(r io.Reader, data []byte) (int, error) {
	bytesWritten, err := io.ReadFull(r, data)

	if bytesWritten == len(data) {
		return bytesWritten, nil
	}
	if bytesWritten < len(data) && bytesWritten > 0 {
		return bytesWritten, err
	}

	if err != nil {
		return 0, err
	}
	return 0, nil
}
