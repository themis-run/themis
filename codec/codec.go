package codec

import (
	"encoding/json"
	"sync"
)

type Codec interface {
	Name() string
	Decode(data []byte, v interface{}) error
	Encode(v interface{}) ([]byte, error)
}

var (
	codecMap         = make(map[string]Codec)
	lock             sync.Mutex
	defaultCodecName = Json
)

const (
	Json = "json"
	Gob  = "gob"
)

func Get(name string) Codec {
	c, ok := codecMap[name]
	if ok {
		return c
	}

	return &codec{name: defaultCodecName}
}

func Register(name string, codec Codec) {
	lock.Lock()
	defer lock.Unlock()

	if codecMap == nil {
		codecMap = make(map[string]Codec)
	}
	codecMap[name] = codec
}

func init() {
	codecMap[defaultCodecName] = &codec{name: defaultCodecName}
}

type codec struct {
	name string
}

func (c *codec) Name() string {
	return c.name
}

func (c *codec) Decode(data []byte, v interface{}) error {
	err := json.Unmarshal(data, v)
	return err
}

func (c *codec) Encode(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}
