package store

import "github.com/cornelk/hashmap"

type KV interface {
	Set(string, *Node) *Event
	Get(string) (*Event, bool)
	Delete(string) *Event
	Iter() <-chan Node
}

func newKVStore(size uintptr) KV {
	return &kvstore{
		m: hashmap.New(size),
	}
}

type kvstore struct {
	m *hashmap.HashMap
}

func (kv *kvstore) Set(key string, value *Node) *Event {
	event := &Event{
		Name: Set,
		Node: value,
	}

	v, ok := kv.m.Get(key)
	if ok {
		event.OldNode = v.(*Node)
	}
	kv.m.Set(key, value)

	return event
}

func (kv *kvstore) Get(key string) (*Event, bool) {
	v, ok := kv.m.Get(key)
	n := v.(*Node)
	if n.Expired || n.IsExpire() {
		return nil, false
	}

	return &Event{
		Name: Get,
		Node: n,
	}, ok
}

func (kv *kvstore) Delete(key string) *Event {
	event := &Event{
		Name: Delete,
	}

	v, ok := kv.m.Get(key)
	if ok {
		event.OldNode = v.(*Node)
		kv.m.Del(key)
	}

	return event
}

func (kv *kvstore) Iter() <-chan Node {
	itr := kv.m.Iter()
	nodeList := make([]Node, 0)
	ch := make(chan Node)

	for keyValue := range itr {
		n := keyValue.Value.(Node)
		nodeList = append(nodeList, n)
	}

	go func() {
		for _, v := range nodeList {
			ch <- v
		}
		close(ch)
	}()

	return ch
}
