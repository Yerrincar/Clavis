package networking

import (
	"Clavis/src/command"
	"Clavis/src/resp"
)

type Handler struct {
	resp     *resp.RespSVC
	Dispatch *command.Service
}

func NewHandler() *Handler {
	return &Handler{
		resp:     resp.NewRespService(),
		Dispatch: command.NewService(),
	}
}
