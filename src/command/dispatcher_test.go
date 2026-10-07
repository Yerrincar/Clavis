package command

import (
	"testing"

	sds "Clavis/src/dataStructures"
)

func commandSDS(t *testing.T, value string) *sds.SDS {
	t.Helper()

	result, err := sds.NewSDS(len(value))
	if err != nil {
		t.Fatalf("NewSDS() error = %v", err)
	}
	buf, err := result.AvailableWritableRegion(len(value))
	if err != nil {
		t.Fatalf("AvailableWritableRegion() error = %v", err)
	}
	copy(buf, value)
	if err := result.IncrLen(len(value)); err != nil {
		t.Fatalf("IncrLen() error = %v", err)
	}

	return result
}

func TestExecuteRejectsWrongGetArity(t *testing.T) {
	reply := NewService().Execute([]*sds.SDS{commandSDS(t, "GET")})

	if reply.Type != TypeError {
		t.Fatalf("Execute() reply type = %v, want %v", reply.Type, TypeError)
	}
	if got, want := reply.Err.Error(), "wrong number of arguments for 'get' command"; got != want {
		t.Fatalf("Execute() error = %q, want %q", got, want)
	}
}
