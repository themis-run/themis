package store

import "github.com/cornelk/hashmap"

type KV interface {
	Set(string, *Node) *Event
	Get(string) (*Event, bool)
	Delete(string) *Event
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
	return &Event{
		Name: Get,
		Node: v.(*Node),
	}, ok
}

func (kv *kvstore) Delete(key string) *Event {
	event := &Event{
		Name: Set,
	}

	v, ok := kv.m.Get(key)
	if ok {
		event.OldNode = v.(*Node)
	}
	kv.m.Del(key)

	return event
}
