package resp

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"math"
	"strconv"
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

var (
	emptyInputError          = errors.New("Input data is empty")
	invalidInputSimpleString = errors.New("Input data is invalid for simple string")
	invalidInputSimpleError  = errors.New("Input data is invalid for simple error")
	invalidInputInteger      = errors.New("Input data is invalid for integer")
	invalidInputBulk         = errors.New("Input data is invalid for bulks")
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
	case ERROR:
		return r.ParseSimpleError
	case INTEGER:
		return r.ParseInteger
	case BULK:
		return r.ParseBulk
	default:
		return func(dt *DataType) error { return errors.New("invalid input data") }
	}
}

func (r *RespSVC) ParseSimpleString(dataType *DataType) error {
	return r.ParseSimpleInput(dataType, invalidInputSimpleString)
}

func (r *RespSVC) ParseSimpleError(dataType *DataType) error {
	return r.ParseSimpleInput(dataType, invalidInputSimpleError)
}

func (r *RespSVC) ParseSimpleInput(dataType *DataType, invalidError error) error {
	reader := bufio.NewReader(bytes.NewReader(dataType.Msg[1:]))
	var res strings.Builder

	err := r.readUntilCRLF(reader, func(b byte) error {
		res.WriteString(string(b))
		return nil
	}, invalidError)

	if err != nil {
		return err
	}

	dataType.ReturnType = res.String()
	return nil
}

func (r *RespSVC) ParseInteger(dataType *DataType) error {
	//reader := bufio.NewReader(bytes.NewReader(dataType.Msg[1:]))
	var reader *bufio.Reader
	signByte := dataType.Msg[1]
	var sign int64 = 1
	if signByte == ERROR {
		sign *= -1
	}

	if signByte == STRING || signByte == ERROR {
		reader = bufio.NewReader(bytes.NewReader(dataType.Msg[2:]))
	} else {
		reader = bufio.NewReader(bytes.NewReader(dataType.Msg[1:]))
	}

	var result int64
	foundDigit := false

	err := r.readUntilCRLF(reader, func(b byte) error {
		if b < '0' || b > '9' {
			return invalidInputInteger
		}

		if !foundDigit {
			foundDigit = true
		}

		if sign > 0 {
			if result > (math.MaxInt64-int64(b-'0'))/10 {
				return invalidInputInteger
			}
			result = result*10 + int64(b-'0')
		} else {
			if result < (math.MaxInt64-int64(b-'0'))/10 {
				return invalidInputInteger
			}
			result = result*10 + int64(b-'0')
		}
		return nil
	}, invalidInputInteger)

	if err != nil {
		return err
	}

	if !foundDigit {
		return invalidInputInteger
	}

	dataType.ReturnType = result
	return nil
}

func (r *RespSVC) ParseBulk(dataType *DataType) error {
	size := len(dataType.Msg)

	if size < 4 || size > 512*1024*1024 {
		return invalidInputBulk
	}

	startingByte := bytes.IndexByte(dataType.Msg, CR)
	if startingByte == -1 {
		return invalidInputBulk
	}

	if startingByte+1 > size-1 || dataType.Msg[startingByte+1] != LF {
		return invalidInputBulk
	}

	length, err := strconv.Atoi(string(dataType.Msg[1:startingByte]))
	if err != nil {
		return err
	}

	if length == -1 {
		dataType.ReturnType = nil
		return nil
	}

	if length < -1 {
		return invalidInputBulk
	}

	bulkStart := startingByte + 2
	bulkEnd := bulkStart + length

	if bulkEnd+2 > size {
		return invalidInputBulk
	}

	if string(dataType.Msg[bulkEnd:bulkEnd+2]) != "\r\n" {
		return invalidInputBulk
	}

	dataType.ReturnType = dataType.Msg[bulkStart:bulkEnd]
	return nil
}

func (r *RespSVC) readUntilCRLF(reader *bufio.Reader, handler func(byte) error, invalidError error) error {
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
				return invalidError
			}

			if foundCR && b == CR {
				return invalidError
			}

			if foundLF && b == LF {
				return invalidError
			}

			if !foundCR && b == CR {
				foundCR = true
			}

			if !foundLF && b == CR {
				foundLF = true
			}

			if foundLF && !foundCR {
				return invalidError
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
		return invalidError
	}
	return nil
}
