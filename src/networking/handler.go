package networking

import (
	sds "Clavis/src/dataStructures"
	"Clavis/src/resp"
)

type Handler struct {
	resp *resp.RespSVC
	sds  *sds.SDS
}
