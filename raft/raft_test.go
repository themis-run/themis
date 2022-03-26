package raft

import (
	"fmt"
	"net"
	"testing"
	"time"

	"go.themis.run/themis/logging"
	"google.golang.org/grpc"
)

func createRaftPeers() map[string]string {
	return map[string]string{
		"a": "localhost:50051",
		"b": "localhost:50052",
		"c": "localhost:50053",
	}
}

func runTestRaftServer(raft *Raft, peersConfig map[string]string) {
	lis, err := net.Listen("tcp", peersConfig[raft.me])
	if err != nil {
		logging.Fatal(err)
	}

	s := grpc.NewServer()
	RegisterRaftServer(s, raft)

	go func() {
		if err := s.Serve(lis); err != nil {
			logging.Fatal(err)
		}
	}()
}

func newTestRaftClient(name string, peersConfig map[string]string) map[string]RaftClient {
	peers := make(map[string]RaftClient)
	var err error
	for k, v := range peersConfig {
		if k == name {
			continue
		}

		peers[k], err = newClient(v)
		if err != nil {
			logging.Fatal(err)
		}
	}
	return peers
}

func newTestRaft(name string) *Raft {
	applyCh := make(chan ApplyMsg, 10)
	persister := MakePersister()

	opt := DefaultOptions()
	opt.NativeName = name

	return NewRaft(persister, applyCh, opt)
}

func TestRaftElection(t *testing.T) {
	m := createRaftPeers()

	raftmap := make(map[string]*Raft, 0)

	for k := range m {
		raftmap[k] = newTestRaft(k)
		runTestRaftServer(raftmap[k], m)
		peers := newTestRaftClient(k, m)
		raftmap[k].Start(peers)
	}

	time.Sleep(5 * time.Second)

	leader := findLeader(raftmap)
	if leader == "" {
		t.Fatal()
	}
}

func TestAppendLog(t *testing.T) {
	m := createRaftPeers()

	raftmap := make(map[string]*Raft, 0)

	for k := range m {
		raftmap[k] = newTestRaft(k)
		runTestRaftServer(raftmap[k], m)
		peers := newTestRaftClient(k, m)
		raftmap[k].Start(peers)
	}

	time.Sleep(2 * time.Second)

	name := findLeader(raftmap)
	if name == "" {
		t.Fail()
	}

	logging.Info(name)
	logging.Info(raftmap[name].Put([]byte("sdafgd")))

	logging.Info(name)
	logging.Info(raftmap[name].Put([]byte("sdafgd")))

	time.Sleep(2 * time.Second)

	res := make(map[string][]ApplyMsg)
	for k := range raftmap {
		res[k] = make([]ApplyMsg, 0)
		length := len(raftmap[k].applyCh)
		for i := 0; i < length; i++ {
			res[k] = append(res[k], <-raftmap[k].applyCh)
		}
	}

	for k := range res {
		logging.Info(res[k])
	}
}

func findLeader(raftmap map[string]*Raft) string {
	leader := ""
	for k := range raftmap {
		fmt.Printf("name: %s role %s\n", k, raftmap[k].role)
		if raftmap[k].role == Leader {
			if leader != "" {
				fmt.Printf("Leader: %s  %s\n", leader, k)
				return ""
			}
			leader = k
		}
	}
	return leader
}
