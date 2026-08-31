package object

import sds "Clavis/src/dataStructures"

type obj struct {
	sds *sds.SDS
}

func (o *obj) trimStringObjectIfNeeded() {
	len := o.sds.Len()
	free := o.sds.Available()

	if free > len/10 {
		o.sds.RemoveFreeSpace()
	}
}
