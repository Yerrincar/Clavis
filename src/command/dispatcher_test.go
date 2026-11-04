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

func TestExecutePingReplies(t *testing.T) {
	service := NewService()

	if reply := service.Execute([]*sds.SDS{commandSDS(t, "PING")}); reply.Type != TypeStatus || reply.String != "PONG" {
		t.Fatalf("PING reply = %#v, want PONG status", reply)
	}

	message := commandSDS(t, "hello")
	if reply := service.Execute([]*sds.SDS{commandSDS(t, "PING"), message}); reply.Type != TypeBulk || string(reply.Bytes.BorrowBytes()) != "hello" {
		t.Fatalf("PING message reply = %#v, want hello Bulk String", reply)
	}
}

func TestExecuteSetAcceptsOptionalTTL(t *testing.T) {
	service := NewService()

	for _, command := range [][]*sds.SDS{
		{commandSDS(t, "SET"), commandSDS(t, "persistent"), commandSDS(t, "value")},
		{commandSDS(t, "SET"), commandSDS(t, "temporary"), commandSDS(t, "value"), commandSDS(t, "1h")},
	} {
		if reply := service.Execute(command); reply.Type != TypeStatus || reply.String != "OK" {
			t.Fatalf("SET reply = %#v, want OK status", reply)
		}
	}
}
