package object

import ds "Clavis/src/dataStructures"

type obj struct {
	sds *ds.SDS
}

func (o *obj) trimStringObjectIfNeeded() {
	len := o.sds.Len()
	free := o.sds.Available()

	if free > len/10 {
		o.sds.RemoveFreeSpace()
	}
}
