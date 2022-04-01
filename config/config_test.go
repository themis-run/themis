package config

import (
	"testing"
)

func TestCreate(t *testing.T) {
	conf := Create("")

	if conf.Name != "test" {
		t.Fail()
	}

	if conf.Address != "localhost:8080" {
		t.Fail()
	}

	if conf.Size != 16 {
		t.Fail()
	}

	if len(conf.Raft.RaftPeers) != 2 {
		t.Fail()
	}
}
