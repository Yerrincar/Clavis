package resp

import (
	"bytes"
	"testing"

	sds "Clavis/src/dataStructures"
)

func TestParseArrayEmpty(t *testing.T) {
	parser := NewRespService()
	dataType, err := parser.Parse([]byte("*0\r\n"))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	count, header, err := parser.ParseArray(&dataType, 0)
	if err != nil {
		t.Fatalf("ParseArray() error = %v", err)
	}
	if count != 0 || header != len("*0\r\n") {
		t.Fatalf("ParseArray() = (%d, %d), want (0, 4)", count, header)
	}
}

func TestSerializeBulkString(t *testing.T) {
	value, err := sds.NewSDS(3)
	if err != nil {
		t.Fatalf("NewSDS() error = %v", err)
	}
	buf, err := value.AvailableWritableRegion(3)
	if err != nil {
		t.Fatalf("AvailableWritableRegion() error = %v", err)
	}
	copy(buf, "bar")
	if err := value.IncrLen(3); err != nil {
		t.Fatalf("IncrLen() error = %v", err)
	}

	parts, err := NewRespService().SerializeBulkString(value)
	if err != nil {
		t.Fatalf("SerializeBulkString() error = %v", err)
	}
	if got, want := string(bytes.Join(parts, nil)), "$3\r\nbar\r\n"; got != want {
		t.Fatalf("SerializeBulkString() = %q, want %q", got, want)
	}
}
