package raft

type Server interface {
	Put([]byte) bool
	CommitChannel() <-chan []byte
	Run()
	Kill()
}

type server struct {
	raft     *Raft
	option   *Options
	commitCh chan []byte
	stopch   chan struct{}
}

func New(opts ...Option) Server {
	o := DefaultOptions()

	for _, op := range opts {
		op(o)
	}

	applyCh := make(chan ApplyMsg, o.ApplyMsgLength)
	persister := MakePersister()
	r := NewRaft(persister, applyCh, o)

	commitCh := make(chan []byte, o.ApplyMsgLength)
	stopch := make(chan struct{}, 1)

	return &server{
		raft:     r,
		option:   o,
		commitCh: commitCh,
		stopch:   stopch,
	}
}

func (s *server) CommitChannel() <-chan []byte {
	return s.commitCh
}

func (s *server) Run() {
	peers := make(map[string]RaftClient)
	for k, v := range s.option.RaftPeers {
		c, err := newClient(v)
		if err != nil {
			continue
		}

		peers[k] = c
	}

	go s.listenApplyMsg()

	s.raft.Start(peers)
}

func (s *server) Put(commend []byte) bool {
	_, _, isLeader := s.raft.Put(commend)
	return isLeader
}

func (s *server) Kill() {
	s.stopch <- struct{}{}
	s.raft.Kill()
}

func (s *server) listenApplyMsg() {
	for !s.raft.killed() {
		select {
		case <-s.stopch:
			break
		case msg := <-s.raft.ApplyChan():
			if msg.CommandValid {
				s.commitCh <- msg.Command
				continue
			}

			switch string(msg.Command) {
			// raft log read to store
			case InstallSnapshotToStore:

			case AddMemeberToRaft:

			}
		}
	}
}
