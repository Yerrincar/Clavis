package ds

import "testing"

func TestNewSDSFromBytesOwnsInput(t *testing.T) {
	input := []byte("value")

	value, err := NewSDSFromBytes(input)
	if err != nil {
		t.Fatalf("NewSDSFromBytes() error = %v", err)
	}
	input[0] = 'X'

	if got, want := string(value.BorrowBytes()), "value"; got != want {
		t.Fatalf("NewSDSFromBytes() value = %q, want %q", got, want)
	}
}
