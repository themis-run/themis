package store

type Action string

const (
	All    = "all"
	Set    = "set"
	Get    = "get"
	Delete = "delete"
	Expire = "expire"
)

type watchEvent struct {
	action Action
}

type watcherHub struct {
}
