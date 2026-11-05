package store

type Limit struct {
	MaxBytes int64
}

func NewLimit() *Limit {
	return &Limit{
		//hardcoded, change later
		MaxBytes: 5 << 30,
	}
}
