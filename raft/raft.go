package raft

import (
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"go.themis.run/themis/codec"
)

func init() {
	rand.Seed(time.Now().Unix())
}

const (
	ElectionTimeout  = time.Millisecond * 300 // 选举
	HeartBeatTimeout = time.Millisecond * 150 // leader 发送心跳
	ApplyInterval    = time.Millisecond * 100 // apply log
	RPCTimeout       = time.Millisecond * 100
	MaxLockTime      = time.Millisecond * 10
)

type Role int

const (
	Follower  Role = 0
	Candidate Role = 1
	Leader    Role = 2
)

type ApplyMsg struct {
	CommandValid bool
	CommandIndex int32
	Command      interface{}
}

type Raft struct {
	mu        sync.RWMutex
	me        string
	peers     map[string]RaftClient
	persister *Persister
	dead      int32

	role Role
	term int32

	electionTimer       *time.Timer
	appendEntriesTimers []*time.Timer
	applyTimer          *time.Timer
	notifyApplyCh       chan struct{}
	stopCh              chan struct{}

	voteFor           string
	logEntries        []*LogEntry
	applyCh           chan ApplyMsg
	commitIndex       int32
	lastSnapshotIndex int32
	lastSnapshotTerm  int32
	lastApplied       int32
	nextIndex         map[string]int32
	matchIndex        map[string]int32
	coder             codec.Codec

	UnimplementedRaftServer
}

type raftState struct {
	Term              int32
	VoteFor           string
	CommitIndex       int32
	LastSnapshotIndex int32
	LastSnapshotTerm  int32
	LogEntries        []*LogEntry
}

func (rf *Raft) getRaftState() *raftState {
	return &raftState{
		Term:              rf.term,
		VoteFor:           rf.voteFor,
		CommitIndex:       rf.commitIndex,
		LastSnapshotIndex: rf.lastSnapshotIndex,
		LastSnapshotTerm:  rf.lastSnapshotTerm,
		LogEntries:        rf.logEntries,
	}
}

func (rf *Raft) loadRaftState(r *raftState) {
	rf.term = r.Term
	rf.voteFor = r.VoteFor
	rf.commitIndex = r.CommitIndex
	rf.lastSnapshotIndex = r.LastSnapshotIndex
	rf.lastSnapshotTerm = r.LastSnapshotTerm
	rf.logEntries = r.LogEntries
}

func (rf *Raft) getPersistData() ([]byte, error) {
	return rf.coder.Encode(rf.getRaftState())
}

func (rf *Raft) persist() {
	data, err := rf.getPersistData()
	if err != nil {
		// log
		return
	}
	rf.persister.SaveRaftState(data)
}

func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 {
		return
	}

	r := &raftState{}
	if err := rf.coder.Decode(data, r); err != nil {
		log.Fatal("raft read persist error")
	}

	rf.loadRaftState(r)
}

func (rf *Raft) changeRole(role Role) {
	rf.role = role
	switch role {
	case Follower:
	case Candidate:
		rf.term += 1
		rf.voteFor = rf.me
		rf.resetElectionTimer()
	case Leader:
		_, lastLogIndex := rf.lastLogTermIndex()
		rf.nextIndex = make(map[string]int32)
		for k := range rf.peers {
			rf.nextIndex[k] = lastLogIndex + 1
		}

		rf.matchIndex = make(map[string]int32)
		rf.matchIndex[rf.me] = lastLogIndex
		rf.resetElectionTimer()
	default:
		panic("unknown role")
	}
}

func (rf *Raft) lastLogTermIndex() (int32, int32) {
	term := rf.logEntries[len(rf.logEntries)-1].Term
	index := rf.lastSnapshotIndex + int32(len(rf.logEntries)) - 1
	return term, index
}

func randElectionTimeout() time.Duration {
	r := time.Duration(rand.Int63()) % ElectionTimeout
	return ElectionTimeout + r
}

func (rf *Raft) resetElectionTimer() {
	rf.electionTimer.Stop()
	rf.electionTimer.Reset(randElectionTimeout())
}

func (rf *Raft) resetHeartBeatTimers() {
	for i, _ := range rf.appendEntriesTimers {
		rf.appendEntriesTimers[i].Stop()
		rf.appendEntriesTimers[i].Reset(0)
	}
}

func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	// Your code here, if desired.
	close(rf.stopCh)
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

func (rf *Raft) Start(command []byte) (int32, int32, bool) {
	// Your code here (2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()

	term := rf.term
	isLeader := rf.role == Leader
	_, lastIndex := rf.lastLogTermIndex()
	index := lastIndex + 1

	if isLeader {
		rf.logEntries = append(rf.logEntries, &LogEntry{
			Term:    rf.term,
			Command: command,
			Index:   int32(index),
		})
		rf.matchIndex[rf.me] = index
		rf.persist()
	}
	rf.resetHeartBeatTimers()
	return index, term, isLeader
}

func (rf *Raft) startApplyLogs() {
	defer rf.applyTimer.Reset(ApplyInterval)

	rf.mu.Lock()
	var msgs []ApplyMsg
	if rf.lastApplied < rf.lastSnapshotIndex {
		msgs = make([]ApplyMsg, 0, 1)
		msgs = append(msgs, ApplyMsg{
			CommandValid: false,
			Command:      "installSnapShot",
			CommandIndex: rf.lastSnapshotIndex,
		})

	} else if rf.commitIndex <= rf.lastApplied {
		// snapShot 没有更新 commitidx
		msgs = make([]ApplyMsg, 0)
	} else {
		msgs = make([]ApplyMsg, 0, rf.commitIndex-rf.lastApplied)
		for i := rf.lastApplied + 1; i <= rf.commitIndex; i++ {
			msgs = append(msgs, ApplyMsg{
				CommandValid: true,
				Command:      rf.logEntries[rf.getRealIdxByLogIndex(i)].Command,
				CommandIndex: i,
			})
		}
	}
	rf.mu.Unlock()

	for _, msg := range msgs {
		rf.applyCh <- msg
		rf.mu.Lock()
		rf.lastApplied = msg.CommandIndex
		rf.mu.Unlock()
	}
}

func (rf *Raft) getLogByIndex(logIndex int32) *LogEntry {
	idx := logIndex - rf.lastSnapshotIndex
	return rf.logEntries[idx]
}

func (rf *Raft) getRealIdxByLogIndex(logIndex int32) int32 {
	idx := logIndex - rf.lastSnapshotIndex
	if idx < 0 {
		return -1
	} else {
		return idx
	}
}