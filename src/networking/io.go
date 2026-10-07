package networking

import (
	sds "Clavis/src/dataStructures"
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

func WriteAll(w io.Writer, parts [][]byte) error {
	for _, part := range parts {
		for len(part) > 0 {
			n, err := w.Write(part)
			if err != nil {
				return err
			}
			if n == 0 {
				return io.ErrShortWrite
			}
			part = part[n:]
		}
	}

	return nil
}

func (h *Handler) ReadIntoBulkSDS(r io.Reader, sds *sds.SDS, n int) (int, error) {
	availableRegion, err := sds.AvailableWritableRegion(n)
	if err != nil {
		return 0, err
	}

	bytesWritten, err := ReadConn(r, availableRegion)
	if err != nil {
		return 0, err
	}

	err = sds.IncrLen(bytesWritten)
	if err != nil {
		return 0, err
	}

	return bytesWritten, nil
}

func (h *Handler) CopyBufferedBulkIntoSDS(data []byte, sdsP *sds.SDS) error {
	err := sdsP.MakeRoomFor(len(data))
	if err != nil {
		return err
	}

	region, err := sdsP.AvailableWritableRegion(len(data))
	if err != nil {
		return err
	}

	copy(region, data)

	err = sdsP.IncrLen(len(data))
	if err != nil {
		return err
	}

	return nil
}
