package networking

import (
	sds "Clavis/src/dataStructures"
	"Clavis/src/resp"
	"errors"
	"log"
	"net"
)

func (h *Handler) server() error {

	listener, err := net.Listen("tcp", ":8090")
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

func (h *Handler) handleConnection(conn net.Conn) {
	var expected, received, remaining int
	defer conn.Close()

	queryBuff := make([]byte, 0, 4096)
	readTmp := make([]byte, 1024)
	mode := "NORMAL"
	var currentSDS *sds.SDS

	for {
		if mode == "NORMAL" {
			if len(queryBuff) <= 0 {
				n, err := conn.Read(readTmp)
				if err != nil {
					return
				}
				queryBuff = append(queryBuff, readTmp[:n]...)
			}

			for len(queryBuff) > 0 {
				dataType := &resp.DataType{Msg: queryBuff}
				length, header, err := h.resp.ParseBulkHeaderSDS(dataType)

				if errors.Is(err, resp.IncompleteInput) {
					n, err := conn.Read(readTmp)
					if err != nil {
						return
					}
					queryBuff = append(queryBuff, readTmp[:n]...)
					break
				}

				if err != nil {
					return
				}

				if length == -1 {
					currentSDS = nil
				}

				if length == 0 {
					queryBuff = queryBuff[header:]
					mode = "BULK_TRAILER"
				}

				if length != 0 {
					expected = length
					currentSDS, err = sds.NewSDS(length)
					if err != nil {
						return
					}

					available := len(queryBuff) - header
					payloadBytes := min(length, available)

					received = payloadBytes
					remaining = length - received

					//small bulk string can also stay in this mode without affecting performance, stablish large bulk treshold
					queryBuff = queryBuff[header+payloadBytes:]
					err = h.CopyBufferedBulkIntoSDS(queryBuff, currentSDS)
					if err != nil {
						return
					}
				}

				if remaining > 0 {
					//only after \r\n (header) has been read and still need to decide the threshold
					mode = "BULK_BODY"
					break
				} else {
					mode = "BULK_TRAILER"
					break
				}
			}
		}

		if mode == "BULK_BODY" {
			if received < expected {
				bytesReceived, err := h.ReadIntoBulkSDS(conn, currentSDS, remaining)
				if err != nil {
					return
				}
				received += bytesReceived
			}

			remaining = expected - received
			if remaining == 0 {
				mode = "BULK_TRAILER"
			}
		}

		if mode == "BULK_TRAILER" {
			if len(queryBuff) < 2 {
				tmp := make([]byte, 1)
				_, err := conn.Read(tmp)
				if err != nil {
					return
				}
				queryBuff = append(queryBuff, tmp...)
				continue
			}
			if queryBuff[0] != '\r' || queryBuff[1] != '\n' {
				log.Print(errors.New("Invalid input"))
				return
			}
			queryBuff = queryBuff[2:]

			expected = 0
			received = 0
			remaining = 0

			currentSDS = nil
			mode = "NORMAL"
		}
	}
}
