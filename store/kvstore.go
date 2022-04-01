package store

import (
	"github.com/derekparker/trie"
)

type KV interface {
	Set(string, *Node) *Event
	Get(string) (*Event, bool)
	Delete(string) *Event

	ListNodeByPreKey(prefix string) <-chan Node
}

func newKVStore(size uintptr) KV {
	return &kvstore{
		t: trie.New(),
	}
}

type kvstore struct {
	t *trie.Trie
}

func (kv *kvstore) Set(key string, value *Node) *Event {
	event := &Event{
		Name: Set,
		Node: value,
	}

	v, ok := kv.t.Find(key)
	if ok {
		event.OldNode = v.Meta().(*Node)
	}
	kv.t.Add(key, value)

	return event
}

func (kv *kvstore) Get(key string) (*Event, bool) {
	v, ok := kv.t.Find(key)
	n := v.Meta().(*Node)
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

	v, ok := kv.t.Find(key)
	if ok {
		event.OldNode = v.Meta().(*Node)
		kv.t.Remove(key)
	}

	return event
}

func (kv *kvstore) ListNodeByPreKey(prefix string) <-chan Node {
	keys := kv.t.PrefixSearch(prefix)
	ch := make(chan Node)

	go func() {
		for _, key := range keys {
			v, ok := kv.t.Find(key)
			if !ok {
				continue
			}

			ch <- *v.Meta().(*Node)

			close(ch)
		}
	}()

	return ch
}
