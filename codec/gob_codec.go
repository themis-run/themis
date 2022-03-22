package codec

import (
	"bytes"
	"encoding/gob"
)

func init() {
	Register(Gob, &gobCodec{})
}

type gobCodec struct {
	name string
}

func (g gobCodec) Name() string {
	return g.name
}

func (g *gobCodec) Decode(data []byte, v interface{}) error {
	b := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(b)
	return decoder.Decode(v)
}

func (c *gobCodec) Encode(v interface{}) ([]byte, error) {
	b := new(bytes.Buffer)
	encoder := gob.NewEncoder(b)

	err := encoder.Encode(v)
	if err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}
