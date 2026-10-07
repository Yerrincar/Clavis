package networking

import (
	"Clavis/src/command"
	sds "Clavis/src/dataStructures"
	"Clavis/src/resp"
	"errors"
	"log"
	"net"
)

var NullBulkString = errors.New("Null Bulk String returned")

type bulkParseState struct {
	active     bool
	expected   int
	received   int
	remaining  int
	mode       string
	currentSDS *sds.SDS
}

type arrayParseState struct {
	active    bool
	expected  int
	completed int
	arguments []*sds.SDS
	commands  [][]*sds.SDS
}

func NewBulkParseState() *bulkParseState {
	return &bulkParseState{
		mode: "BULK_BODY",
	}
}
func NewArrayParseState() *arrayParseState {
	return &arrayParseState{
		arguments: []*sds.SDS{},
		commands:  [][]*sds.SDS{},
	}
}

func (h *Handler) Server() error {

	listener, err := net.Listen("tcp", ":8000")
	if err != nil {
		return err
	}

	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Error accepting connection: ", err)
			continue
		}

		go h.handleConnection(conn)
	}
}

func (h *Handler) writeReply(conn net.Conn, reply *command.Reply) error {
	switch reply.Type {
	case command.TypeError:
		response, err := h.resp.SerializeSimpleError(reply.Err.Error())
		if err != nil {
			return err
		}
		return WriteAll(conn, [][]byte{response})
	case command.TypeStatus:
		response, err := h.resp.SerializeSimpleString(reply.String)
		if err != nil {
			return err
		}
		return WriteAll(conn, [][]byte{response})
	case command.TypeBulk, command.TypeNull:
		response, err := h.resp.SerializeBulkString(reply.Bytes)
		if err != nil {
			return err
		}
		return WriteAll(conn, response)
	default:
		return errors.New("unsupported reply type")
	}
}

func (h *Handler) handleConnection(conn net.Conn) {
	defer conn.Close()

	queryBuff := make([]byte, 0, 4096)
	readTmp := make([]byte, 1024)
	bulkState := NewBulkParseState()
	arrayState := NewArrayParseState()

	for {
		var err error
		switch {
		case bulkState.active:
			err = h.parseBulk(conn, &queryBuff, readTmp, bulkState, arrayState)
		case arrayState.active && arrayState.completed == arrayState.expected:
			arrayState.completeCommand()
			for len(arrayState.commands) > 0 {
				currentCommand := arrayState.commands[0]
				reply := h.dispatch.Execute(currentCommand)
				if err := h.writeReply(conn, reply); err != nil {
					return
				}
				arrayState.commands[0] = nil
				arrayState.commands = arrayState.commands[1:]

			}
			continue
		default:
			if len(queryBuff) == 0 {
				n, err := conn.Read(readTmp)
				if err != nil {
					return
				}
				queryBuff = append(queryBuff, readTmp[:n]...)
			}
			err = h.parseElement(conn, &queryBuff, readTmp, bulkState, arrayState)
		}

		if err == resp.IncompleteInput {
			n, err := conn.Read(readTmp)
			if err != nil {
				return
			}
			queryBuff = append(queryBuff, readTmp[:n]...)
			continue
		}
		if err != nil {
			return
		}
	}
}

func (s *arrayParseState) completeCommand() {
	s.commands = append(s.commands, s.arguments)
	s.active = false
	s.expected = 0
	s.completed = 0
	s.arguments = nil
}

func (h *Handler) parseElement(conn net.Conn, queryBuff *[]byte, readTmp []byte, bulkState *bulkParseState, arrayState *arrayParseState) error {

	dataType, err := h.resp.Parse(*queryBuff)
	if err == resp.IncompleteInput {
		return resp.IncompleteInput
	}
	if err != nil {
		return err
	}

	if !arrayState.active {
		if dataType.FirstByte != resp.ARRAY {
			return errors.New("commands must be RESP arrays")
		}

		numberArg, header, err := h.resp.ParseArray(&dataType, 0)
		if err != nil {
			return err
		}
		if numberArg < 0 {
			return errors.New("commands cannot be null arrays")
		}

		arrayState.active = true
		arrayState.expected = numberArg
		arrayState.completed = 0
		arrayState.arguments = arrayState.arguments[:0]
		*queryBuff = (*queryBuff)[header:]
		return nil
	}

	if dataType.FirstByte != resp.BULK {
		return errors.New("command arguments must be bulk strings")
	}

	length, header, err := h.resp.ParseBulkHeaderSDS(&dataType)
	if err != nil {
		return err
	}

	err = h.consumeHeader(queryBuff, length, header, bulkState)
	if err == NullBulkString {
		arrayState.arguments = append(arrayState.arguments, nil)
		arrayState.completed++
		return nil
	}
	if err != nil {
		return err
	}

	return h.parseBulk(conn, queryBuff, readTmp, bulkState, arrayState)
}
func (h *Handler) parseBulk(conn net.Conn, queryBuff *[]byte, readTmp []byte, bulkState *bulkParseState, arrayState *arrayParseState) error {
	for {
		if bulkState.mode == "BULK_BODY" {
			if bulkState.received < bulkState.expected {
				bytesReceived, err := h.ReadIntoBulkSDS(
					conn,
					bulkState.currentSDS,
					bulkState.remaining,
				)
				if err != nil {
					return err
				}
				bulkState.received += bytesReceived
			}

			bulkState.remaining = bulkState.expected - bulkState.received
			if bulkState.remaining == 0 {
				bulkState.mode = "BULK_TRAILER"
			}
		}

		if bulkState.mode == "BULK_TRAILER" {
			for len(*queryBuff) < 2 {
				tmp := make([]byte, 1)
				n, err := conn.Read(tmp)
				if err != nil {
					return err
				}
				*queryBuff = append(*queryBuff, tmp[:n]...)
			}

			if (*queryBuff)[0] != '\r' || (*queryBuff)[1] != '\n' {
				log.Print(errors.New("Invalid input"))
				return errors.New("invalid input")
			}

			*queryBuff = (*queryBuff)[2:]

			bulkState.expected = 0
			bulkState.received = 0
			bulkState.remaining = 0
			arrayState.arguments = append(arrayState.arguments, bulkState.currentSDS)
			arrayState.completed++
			bulkState.currentSDS = nil
			bulkState.mode = "BULK_BODY"
			bulkState.active = false
			return nil
		}
	}
}

func (h *Handler) consumeHeader(queryBuff *[]byte, length int, header int, bulkState *bulkParseState) error {
	if length == -1 {
		*queryBuff = (*queryBuff)[header:]
		return NullBulkString
	}

	if length == 0 {
		bulkState.currentSDS = &sds.SDS{}
		*queryBuff = (*queryBuff)[header:]
		bulkState.expected = 0
		bulkState.received = 0
		bulkState.remaining = 0
		bulkState.mode = "BULK_TRAILER"
		bulkState.active = true
		return nil
	}

	currentSDS, err := sds.NewSDS(length)
	if err != nil {
		return err
	}
	bulkState.currentSDS = currentSDS

	available := len(*queryBuff) - header
	payloadBytes := min(length, available)
	payload := (*queryBuff)[header : header+payloadBytes]

	if err := h.CopyBufferedBulkIntoSDS(payload, bulkState.currentSDS); err != nil {
		return err
	}

	*queryBuff = (*queryBuff)[header+payloadBytes:]
	bulkState.expected = length
	bulkState.received = payloadBytes
	bulkState.remaining = bulkState.expected - bulkState.received
	bulkState.mode = "BULK_BODY"
	bulkState.active = true
	return nil
}
