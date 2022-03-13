package store

type Opreation string

const (
	All    = "all"
	Set    = "set"
	Get    = "get"
	Delete = "delete"
	Expire = "expire"
)

type Event struct {
	Name    Opreation
	OldNode *Node
}

type watcherHub struct {
}
