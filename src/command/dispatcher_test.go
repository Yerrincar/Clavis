package command

import (
	"testing"

	ds "Clavis/src/dataStructures"
)

func commandSDS(t *testing.T, value string) *ds.SDS {
	t.Helper()

	result, err := ds.NewSDS(len(value))
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
	reply := NewService().Execute([]*ds.SDS{commandSDS(t, "GET")})

	if reply.Type != TypeError {
		t.Fatalf("Execute() reply type = %v, want %v", reply.Type, TypeError)
	}
	if got, want := reply.Err.Error(), "wrong number of arguments for 'get' command"; got != want {
		t.Fatalf("Execute() error = %q, want %q", got, want)
	}
}

func TestExecutePingReplies(t *testing.T) {
	service := NewService()

	if reply := service.Execute([]*ds.SDS{commandSDS(t, "PING")}); reply.Type != TypeStatus || reply.String != "PONG" {
		t.Fatalf("PING reply = %#v, want PONG status", reply)
	}

	message := commandSDS(t, "hello")
	if reply := service.Execute([]*ds.SDS{commandSDS(t, "PING"), message}); reply.Type != TypeBulk || string(reply.Bytes.BorrowBytes()) != "hello" {
		t.Fatalf("PING message reply = %#v, want hello Bulk String", reply)
	}
}

func TestExecuteSetAcceptsOptionalTTL(t *testing.T) {
	service := NewService()

	for _, command := range [][]*ds.SDS{
		{commandSDS(t, "SET"), commandSDS(t, "persistent"), commandSDS(t, "value")},
		{commandSDS(t, "SET"), commandSDS(t, "temporary"), commandSDS(t, "value"), commandSDS(t, "1h")},
	} {
		if reply := service.Execute(command); reply.Type != TypeStatus || reply.String != "OK" {
			t.Fatalf("SET reply = %#v, want OK status", reply)
		}
	}
}

func TestExecuteStatsReply(t *testing.T) {
	reply := NewService().Execute([]*ds.SDS{commandSDS(t, "STATS")})
	if reply.Type != TypeBulk {
		t.Fatalf("STATS reply type = %v, want %v", reply.Type, TypeBulk)
	}

	want := "Items: 0\nBytes: 0\nHits: 0\nMisses: 0\nExpired: 0\nEvicted: 0\nRejected: 0"
	if got := string(reply.Bytes.BorrowBytes()); got != want {
		t.Fatalf("STATS reply = %q, want %q", got, want)
	}
}
