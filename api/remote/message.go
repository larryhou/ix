package remote

type Value[T any] struct {
	Type int
	Data T
}

type MessageAux struct {
	Values []any
}

func (x *MessageAux) AddU32(v uint32) {
	x.Values = append(x.Values, &Value[uint32]{
		Type: 3,
		Data: v,
	})
}

func (x *MessageAux) AddU64(v uint64) {
	x.Values = append(x.Values, &Value[uint64]{
		Type: 6,
		Data: v,
	})
}

func (x *MessageAux) AddObject(v map[string]any) {
	x.Values = append(x.Values, &Value[map[string]any]{
		Type: 2,
		Data: v,
	})
}

func (x *MessageAux) Bytes() []byte {
	//for _, v := range x.Values {
	//
	//}

	panic(``)
}
