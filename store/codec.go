package store

import (
	"encoding/json"
	"sync"
)

type Codec interface {
	Name() string
	Decode(data []byte) (*Event, error)
	Encode(event *Event) ([]byte, error)
}

var (
	codecMap         = make(map[string]Codec)
	lock             sync.Mutex
	defaultCodecName = "json"
)

func RegisterCodec(name string, codec Codec) {
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

func (c *codec) Decode(data []byte) (*Event, error) {
	event := &Event{}
	err := json.Unmarshal(data, event)
	return event, err
}

func (c *codec) Encode(event *Event) ([]byte, error) {
	return json.Marshal(event)
}
