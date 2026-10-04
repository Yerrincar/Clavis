package resp

import "testing"

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
