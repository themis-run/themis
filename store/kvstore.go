package store

type KV interface {
	Set(string, *Node)
	Get(string) (*Node, bool)
	Delete(string)
}
