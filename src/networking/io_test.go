package networking

import "testing"

type shortWriter struct {
	data  []byte
	limit int
}

func (w *shortWriter) Write(data []byte) (int, error) {
	n := min(w.limit, len(data))
	w.data = append(w.data, data[:n]...)
	return n, nil
}

func TestWriteAllHandlesShortWrites(t *testing.T) {
	writer := &shortWriter{limit: 2}

	if err := WriteAll(writer, [][]byte{[]byte("$3\r\n"), []byte("bar"), []byte("\r\n")}); err != nil {
		t.Fatalf("writeAll() error = %v", err)
	}
	if got, want := string(writer.data), "$3\r\nbar\r\n"; got != want {
		t.Fatalf("writeAll() wrote %q, want %q", got, want)
	}
}
