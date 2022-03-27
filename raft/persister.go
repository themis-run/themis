package raft

import (
	"sync"

	"go.themis.run/themis/logging"
	"go.themis.run/themis/store"
)

const (
	DefaultRaftStateLogName = "raft-state-log"
	DefaultSnapshotLogName  = "raft-snapshot-log"
)

type Persister struct {
	mu           sync.Mutex
	snapshotSize int
	raftstate    []byte

	raftstateReadWriter store.ReadWriter
	snapshotReadWriter  store.ReadWriter
}

func MakePersister() *Persister {
	return &Persister{}
}

func (ps *Persister) SaveRaftState(state []byte) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.raftstate = state

	sequenceNumber := ps.raftstateReadWriter.NextSequenceNumber()
	if _, err := ps.raftstateReadWriter.Write(sequenceNumber, state); err != nil {
		logging.Error(err)
	}
}

func (ps *Persister) ReadRaftState() []byte {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	sequenceNumber := ps.raftstateReadWriter.NextSequenceNumber() - 1
	result, err := ps.raftstateReadWriter.ListAfter(sequenceNumber)
	if err != nil {
		logging.Error(err)
		return nil
	}

	ps.raftstate = result[len(result)-1]
	return ps.raftstate
}

func (ps *Persister) RaftStateSize() int {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return len(ps.raftstate)
}

func (ps *Persister) SaveStateAndSnapshot(state []byte, snapshot [][]byte) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ps.raftstate = state
	raftstateSeqID := ps.raftstateReadWriter.NextSequenceNumber()
	if _, err := ps.raftstateReadWriter.Write(raftstateSeqID, state); err != nil {
		logging.Error(err)
	}

	snapshotSeqID := ps.snapshotReadWriter.NextSequenceNumber()

	for _, entry := range snapshot {
		ps.snapshotReadWriter.Write(snapshotSeqID, entry)
		snapshotSeqID++
	}
	ps.snapshotSize += len(snapshot)
}

func (ps *Persister) SaveSnapshot(snapshot [][]byte) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	snapshotSeqID := ps.snapshotReadWriter.NextSequenceNumber()

	for _, entry := range snapshot {
		ps.snapshotReadWriter.Write(snapshotSeqID, entry)
		snapshotSeqID++
	}
	ps.snapshotSize += len(snapshot)
}

func (ps *Persister) ReadSnapshotByLastLength(lastLength int32) [][]byte {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	snapshotSeqID := ps.snapshotReadWriter.NextSequenceNumber() - uint64(lastLength) - 1

	result, err := ps.snapshotReadWriter.ListAfter(snapshotSeqID)
	if err != nil {
		logging.Error(err)
		return nil
	}

	return result
}

func (ps *Persister) ReadSnapshot() [][]byte {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	snapshotSeqID := ps.snapshotReadWriter.NextSequenceNumber()

	result, err := ps.snapshotReadWriter.ListBefore(snapshotSeqID)
	if err != nil {
		logging.Error(err)
		return nil
	}
	return result
}

func (ps *Persister) SnapshotSize() int {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return ps.snapshotSize
}
