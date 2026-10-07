package resp

import (
	sds "Clavis/src/dataStructures"
	"bufio"
	"bytes"
	"errors"
	"fmt"
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
	EmptyInputError          = errors.New("Input data is empty")
	invalidInputSimpleString = errors.New("Input data is invalid for simple string")
	invalidInputSimpleError  = errors.New("Input data is invalid for simple error")
	invalidInputInteger      = errors.New("Input data is invalid for integer")
	invalidInputBulk         = errors.New("Input data is invalid for bulks")
	invalidInputArray        = errors.New("Input data is invalid for array")
	invalidInputError        = errors.New("Input data is invalid")
	IncompleteInput          = errors.New("Input data is incomplete, need more bytes")
	bulkTrailer              = []byte("\r\n")
	nullBulkString           = []byte("$-1\r\n")
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
	if len(data) == 0 {
		return DataType{}, EmptyInputError
	}

	if len(data) < 3 {
		return DataType{}, IncompleteInput
	}

	if !isKnownType(data[0]) {
		return DataType{}, invalidInputError
	}

	dataType := DataType{
		FirstByte: data[0],
		Msg:       data,
	}

	if dataType.FirstByte == ARRAY {
		if dataType.BulkReturnType == nil {
			dataType.ReturnType = nil
		} else {
			dataType.ReturnType = dataType.BulkReturnType
		}
	}
	return dataType, nil
}

func (r *RespSVC) ParseSimpleString(dataType *DataType) (int, error) {
	return r.ParseSimpleInput(dataType, invalidInputSimpleString)
}

func (r *RespSVC) ParseSimpleError(dataType *DataType) (int, error) {
	return r.ParseSimpleInput(dataType, invalidInputSimpleError)
}

func (r *RespSVC) ParseSimpleInput(dataType *DataType, invalidError error) (int, error) {
	if len(dataType.Msg) < 1 {
		return 0, EmptyInputError
	}

	reader := bufio.NewReader(bytes.NewReader(dataType.Msg[1:]))
	var res strings.Builder

	consumed, err := r.readUntilCRLF(reader, func(b byte) error {
		res.WriteByte(b)
		return nil
	}, invalidError)

	totalConsumed := 1 + consumed
	if err != nil {
		return totalConsumed, err
	}

	dataType.ReturnType = res.String()
	return totalConsumed, nil
}

func (r *RespSVC) ParseInteger(dataType *DataType) (int, error) {
	if len(dataType.Msg) < 2 {
		return 0, IncompleteInput
	}

	prefixLen := 1
	signByte := dataType.Msg[1]
	var sign int64 = 1

	if signByte == ERROR {
		sign = -1
	}

	var reader *bufio.Reader
	if signByte == STRING || signByte == ERROR {
		prefixLen = 2
		reader = bufio.NewReader(bytes.NewReader(dataType.Msg[2:]))
	} else {
		reader = bufio.NewReader(bytes.NewReader(dataType.Msg[1:]))
	}

	var result int64
	foundDigit := false

	consumed, err := r.readUntilCRLF(reader, func(b byte) error {
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
			if result < (math.MinInt64+int64(b-'0'))/10 {
				return invalidInputInteger
			}
			result = result*10 - int64(b-'0')
		}
		return nil
	}, invalidInputInteger)

	totalConsumed := prefixLen + consumed
	if err != nil {
		return totalConsumed, err
	}

	if !foundDigit {
		return totalConsumed, invalidInputInteger
	}

	dataType.ReturnType = result
	return totalConsumed, nil
}

func (r *RespSVC) ParseBulkHeaderSDS(dataType *DataType) (int, int, error) {
	size := len(dataType.Msg)

	if size == 0 || dataType.Msg[0] != BULK {
		return 0, 0, invalidInputBulk
	}

	if size < 4 {
		return 0, 0, IncompleteInput
	}

	startingByte := bytes.IndexByte(dataType.Msg, CR)
	if startingByte == -1 {
		return 0, 0, IncompleteInput
	}

	if startingByte+1 > size-1 || dataType.Msg[startingByte+1] != LF {
		return 0, 0, IncompleteInput
	}

	length, err := strconv.Atoi(string(dataType.Msg[1:startingByte]))
	if err != nil {
		return 0, 0, err
	}

	if length > 512<<20 {
		return 0, 0, errors.New("Length bigger than Max String Size")
	}

	header := startingByte + 2

	if length == -1 {
		dataType.ReturnType = nil
		return -1, header, nil
	}

	if length < -1 {
		return 0, 0, invalidInputBulk
	}

	return length, header, nil
}

func (r *RespSVC) ParseArray(dataType *DataType, pos int) (int, int, error) {
	size := len(dataType.Msg)

	if pos >= len(dataType.Msg) || dataType.Msg[pos] != ARRAY {
		return pos, 0, invalidInputArray
	}

	if size < 3 {
		return pos, 0, IncompleteInput
	}

	startingByte := bytes.IndexByte(dataType.Msg[pos:], CR)

	if startingByte == -1 {
		return pos, 0, IncompleteInput
	}

	if startingByte+1 > size-1 || dataType.Msg[startingByte+1] != LF {
		return pos, 0, IncompleteInput
	}

	numberArr, err := strconv.Atoi(string(dataType.Msg[pos+1 : pos+startingByte]))
	if err != nil {
		return pos, 0, err
	}

	if numberArr == -1 {
		dataType.ReturnType = nil
		return -1, pos + startingByte + 2, nil
	}

	if numberArr < -1 {
		return pos, 0, invalidInputArray
	}

	if numberArr == 0 {
		return 0, pos + startingByte + 2, nil

	}
	header := startingByte + 2

	return numberArr, header, nil
}

func (r *RespSVC) readUntilCRLF(reader *bufio.Reader, handler func(byte) error, invalidError error) (int, error) {
	var consumed int
	var foundCR bool

	for {
		b, err := reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				return consumed, IncompleteInput
			}
			return consumed, err
		}

		consumed++

		if foundCR {
			if b == LF {
				return consumed, nil
			}
			return consumed, invalidError
		}

		if b == CR {
			foundCR = true
			continue
		}

		if b == LF {
			return consumed, invalidError
		}

		if err := handler(b); err != nil {
			return consumed, err
		}
	}
}

func isKnownType(b byte) bool {
	switch b {
	case INTEGER, STRING, BULK, ARRAY, ERROR:
		return true
	default:
		return false
	}
}

func (r *RespSVC) SerializeSimpleString(message string) ([]byte, error) {
	if message == "" {
		return nil, invalidInputSimpleError
	}

	if strings.Contains(message, "\r") || strings.Contains(message, "\n") {
		return nil, invalidInputSimpleString
	}

	response := make([]byte, 0, len(message)+3)
	response = append(response, STRING)
	response = append(response, message...)
	return append(response, CR, LF), nil
}

func (r *RespSVC) SerializeSimpleError(message string) ([]byte, error) {
	if message == "" {
		return nil, invalidInputSimpleError
	}

	if strings.Contains(message, "\r") || strings.Contains(message, "\n") {
		return nil, invalidInputSimpleError
	}

	response := make([]byte, 0, len(message)+3)
	response = append(response, ERROR)
	response = append(response, message...)
	return append(response, CR, LF), nil
}

func (r *RespSVC) SerializeInteger(resp DataType) (string, error) {
	if resp.ReturnType == nil {
		return "", invalidInputInteger
	}

	var num int64
	switch v := resp.ReturnType.(type) {
	case int64:
		num = v
	default:
		return "", invalidInputInteger
	}

	return fmt.Sprintf(":%d\r\n", num), nil
}

func (r *RespSVC) SerializeBulkString(value *sds.SDS) ([][]byte, error) {
	if value == nil {
		return [][]byte{nullBulkString}, nil
	}

	if value.Len() > 512*1024*1024 {
		return nil, invalidInputBulk
	}

	header := make([]byte, 0, 16)
	header = append(header, BULK)
	header = strconv.AppendInt(header, int64(value.Len()), 10)
	header = append(header, CR, LF)

	return [][]byte{header, value.BorrowBytes(), bulkTrailer}, nil
}
