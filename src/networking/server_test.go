package networking

import (
	"net"
	"testing"

	sds "Clavis/src/dataStructures"
	"Clavis/src/resp"
)

func TestParseElementWaitsForFirstArrayElement(t *testing.T) {
	handler := &Handler{resp: resp.NewRespService()}
	queryBuffer := []byte("*3\r\n")
	bulkState := NewBulkParseState()
	arrayState := NewArrayParseState()

	err := handler.parseElement(nil, &queryBuffer, nil, bulkState, arrayState)
	if err != nil {
		t.Fatalf("parseElement() error = %v", err)
	}
	if !arrayState.active || arrayState.expected != 3 {
		t.Fatalf("array state = (active: %t, expected: %d), want (true, 3)", arrayState.active, arrayState.expected)
	}
	if len(queryBuffer) != 0 {
		t.Fatalf("query buffer = %q, want empty", queryBuffer)
	}
}

func TestParseBulkCompletesSplitTrailer(t *testing.T) {
	payload, err := sds.NewSDS(3)
	if err != nil {
		t.Fatalf("NewSDS() error = %v", err)
	}

	client, peer := net.Pipe()
	defer client.Close()
	go func() {
		_, _ = peer.Write([]byte("\n"))
		_ = peer.Close()
	}()

	queryBuffer := []byte("\r")
	bulkState := &bulkParseState{
		active:     true,
		expected:   3,
		received:   3,
		remaining:  0,
		mode:       "BULK_TRAILER",
		currentSDS: payload,
	}
	arrayState := NewArrayParseState()

	if err := (&Handler{}).parseBulk(client, &queryBuffer, nil, bulkState, arrayState); err != nil {
		t.Fatalf("parseBulk() error = %v", err)
	}
	if len(queryBuffer) != 0 {
		t.Fatalf("query buffer = %q, want empty", queryBuffer)
	}
	if arrayState.completed != 1 || len(arrayState.arguments) != 1 {
		t.Fatalf("completed arguments = (%d, %d), want (1, 1)", arrayState.completed, len(arrayState.arguments))
	}
	if arrayState.arguments[0] != payload {
		t.Fatal("parseBulk() did not append the completed SDS")
	}
	if bulkState.active {
		t.Fatal("parseBulk() left the Bulk receiver active")
	}
}

func TestParseElementRejectsNonBulkCommandArgument(t *testing.T) {
	handler := &Handler{resp: resp.NewRespService()}
	queryBuffer := []byte("+OK\r\n")
	bulkState := NewBulkParseState()
	arrayState := &arrayParseState{active: true, expected: 1}

	if err := handler.parseElement(nil, &queryBuffer, nil, bulkState, arrayState); err == nil {
		t.Fatal("parseElement() accepted a non-Bulk command argument")
	}
}

func TestCompleteCommandTransfersArguments(t *testing.T) {
	argument, err := sds.NewSDS(1)
	if err != nil {
		t.Fatalf("NewSDS() error = %v", err)
	}

	state := &arrayParseState{
		active:    true,
		expected:  1,
		completed: 1,
		arguments: []*sds.SDS{argument},
	}
	state.completeCommand()

	if state.active || state.expected != 0 || state.completed != 0 || state.arguments != nil {
		t.Fatal("completeCommand() did not reset the active Array state")
	}
	if len(state.commands) != 1 || state.commands[0][0] != argument {
		t.Fatal("completeCommand() did not retain the completed command")
	}
}

func TestParseBulkCommandArray(t *testing.T) {
	handler := &Handler{resp: resp.NewRespService()}
	queryBuffer := []byte("*2\r\n$3\r\nGET\r\n$3\r\nfoo\r\n")
	bulkState := NewBulkParseState()
	arrayState := NewArrayParseState()

	if err := handler.parseElement(nil, &queryBuffer, nil, bulkState, arrayState); err != nil {
		t.Fatalf("parseElement() array header error = %v", err)
	}
	if err := handler.parseElement(nil, &queryBuffer, nil, bulkState, arrayState); err != nil {
		t.Fatalf("parseElement() command name error = %v", err)
	}
	if err := handler.parseElement(nil, &queryBuffer, nil, bulkState, arrayState); err != nil {
		t.Fatalf("parseElement() argument error = %v", err)
	}

	if arrayState.completed != arrayState.expected {
		t.Fatalf("completed arguments = %d, want %d", arrayState.completed, arrayState.expected)
	}
	if len(arrayState.arguments) != 2 {
		t.Fatalf("argument count = %d, want 2", len(arrayState.arguments))
	}
	if command, key := string(arrayState.arguments[0].BorrowBytes()), string(arrayState.arguments[1].BorrowBytes()); command != "GET" || key != "foo" {
		t.Fatalf("arguments = (%q, %q), want (GET, foo)", command, key)
	}
}
