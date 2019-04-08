package pbutil

import "fmt"

type Marshaler interface {
	Marshal() (data []byte, err error)
}

type Unmarshaler interface {
	Unmarshal(data []byte) error
}

func MustMarshal(m Marshaler) []byte {
	d, err := m.Marshal()
	if err != nil {
		panic(fmt.Sprintf("marshal should never fail (%v)", err))
	}
	return d
}

func MustUnmarshal(um Unmarshaler, data []byte) {
	if err := um.Unmarshal(data); err != nil {
		panic(fmt.Sprintf("unmarshal should never fail (%v)", err))
	}
}

func MaybeUnmarshal(um Unmarshaler, data []byte) bool {
	if err := um.Unmarshal(data); err != nil {
		return false
	}
	return true
}
