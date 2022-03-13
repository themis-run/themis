package store

import "github.com/cornelk/hashmap"

type KV interface {
	Set(string, *Node)
	Get(string) (*Node, bool)
	Delete(string)
}

type kvstore struct {
	m *hashmap.HashMap
}

func (kv *kvstore) Set(key string, value *Node) {
	kv.m.Set(key, value)
}

func (kv *kvstore) Get(key string) (*Node, bool) {
	v, ok := kv.m.Get(key)
	return v.(*Node), ok
}

func (kv *kvstore) Delete(key string) {
	kv.m.Del(key)
}
