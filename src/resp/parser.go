package resp

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"
)

/*
	Supports the following RESP2 data types:

	| RESP data type  | Minimal protocol version | Category   | First byte |
	|-----------------|--------------------------|------------|------------|
	| Simple strings  | RESP2                    | Simple     | +          |
	| Simple Errors   | RESP2                    | Simple     | -          |
	| Integers        | RESP2                    | Simple     | :          |
	| Bulk strings    | RESP2                    | Aggregate  | $          |
	| Arrays          | RESP2                    | Aggregate  | *          |

*/

const (
	INTEGER byte = 58
	STRING  byte = 43
	BULK    byte = 36
	ERROR   byte = 45
	ARRAY   byte = 42
	CR      byte = 13
	LF      byte = 10
)

type DataType struct {
	FirstByte      byte
	Msg            []byte
	ReturnType     any
	BulkReturnType []any
}

type RespSVC struct {
}

func NewRespService() *RespSVC {
	return &RespSVC{}
}

func (r *RespSVC) Parse(data []byte) (DataType, error) {

	dataType := DataType{
		FirstByte: data[0],
		Msg:       data,
	}

	err := r.handleParsing(dataType.FirstByte)(&dataType)
	if err != nil {
		return DataType{}, err
	}

	return DataType{}, nil
}

func (r *RespSVC) handleParsing(dataTypeID byte) func(*DataType) error {
	switch dataTypeID {
	case STRING:
		return r.ParseSimpleString
	default:
		return func(dt *DataType) error { return errors.New("invalid input data") }
	}
}

func (r *RespSVC) ParseSimpleString(dataType *DataType) error {
	reader := bufio.NewReader(bytes.NewReader(dataType.Msg[1:]))
	var res strings.Builder

	err := r.readUntilCRLF(reader, func(b byte) error {
		res.WriteString(string(b))
		return nil
	})

	if err != nil {
		return err
	}

	dataType.ReturnType = res.String()

	return nil
}

func (r *RespSVC) readUntilCRLF(reader *bufio.Reader, handler func(byte) error) error {
	buffer := make([]byte, 5)
	foundLF, foundCR := false, false

	for {
		size, err := reader.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		for i := range size {
			b := buffer[i]

			if foundCR && foundLF {
				return err
			}

			if foundCR && b == CR {
				return err
			}

			if foundLF && b == LF {
				return err
			}

			if !foundCR && b == CR {
				foundCR = true
			}

			if !foundLF && b == CR {
				foundLF = true
			}

			if foundLF && !foundCR {
				return err
			}

			if b == LF || b == CR {
				continue
			}

			if err = handler(b); err != nil {
				return err
			}
		}
	}

	if !foundCR || !foundLF {
		return errors.New("Error")
	}
	return nil
}
