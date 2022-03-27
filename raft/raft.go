package raft

import (
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"go.themis.run/themis/codec"
	"go.themis.run/themis/logging"
	"google.golang.org/protobuf/proto"
)

func init() {
	rand.Seed(time.Now().Unix())
}

type Role string

const (
	Follower  Role = "Follwer"
	Candidate Role = "Candidate"
	Leader    Role = "Leader"
)

type ApplyMsg struct {
	CommandValid bool
	CommandIndex int32
	Command      []byte
}

type Raft struct {
	mu        sync.RWMutex
	me        string
	peers     map[string]RaftClient
	persister *Persister
	dead      int32

	role Role
	term int32

	electionTimeout  time.Duration
	heartbeatTimeout time.Duration
	applyInterval    time.Duration
	rpcTimeout       time.Duration

	electionTimer       *time.Timer
	appendEntriesTimers map[string]*time.Timer
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

func (rf *Raft) loadOption(o *Options) {
	rf.me = o.NativeName
	rf.electionTimeout = o.ElectionTimeout
	rf.heartbeatTimeout = o.ElectionTimeout
	rf.applyInterval = o.ApplyInterval
	rf.rpcTimeout = o.RPCTimeout
}

type RaftState struct {
	Term              int32
	VoteFor           string
	CommitIndex       int32
	LastSnapshotIndex int32
	LastSnapshotTerm  int32
	LogEntries        []*LogEntry
}

const (
	InstallSnapshotToStore = "installSnapShot"
	AddMemeberToRaft       = "addMemberToRaft"
)

func (rf *Raft) getRaftBootstrapState() *RaftState {
	return &RaftState{
		Term:              0,
		VoteFor:           "",
		CommitIndex:       0,
		LastSnapshotIndex: 0,
		LastSnapshotTerm:  0,
		LogEntries: []*LogEntry{
			{
				Term:    0,
				Index:   0,
				Command: nil,
			},
		},
	}
}

func (rf *Raft) getRaftState() *RaftState {
	return &RaftState{
		Term:              rf.term,
		VoteFor:           rf.voteFor,
		CommitIndex:       rf.commitIndex,
		LastSnapshotIndex: rf.lastSnapshotIndex,
		LastSnapshotTerm:  rf.lastSnapshotTerm,
		LogEntries:        rf.logEntries,
	}
}

func (rf *Raft) loadRaftState(r *RaftState) {
	rf.term = r.Term
	rf.voteFor = r.VoteFor
	rf.commitIndex = r.CommitIndex
	rf.lastSnapshotIndex = r.LastSnapshotIndex
	rf.lastSnapshotTerm = r.LastSnapshotTerm
	if r.LogEntries != nil {
		rf.logEntries = r.LogEntries
	}
	rf.logEntries = []*LogEntry{{
		Term:    0,
		Index:   0,
		Command: nil,
	}}
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

func (rf *Raft) ReadSnapshotToLogEntryByLastLength(lastLength int32) []*LogEntry {
	logEntries := make([]*LogEntry, 0)

	res := rf.persister.ReadSnapshotByLastLength(lastLength)
	for _, v := range res {
		entry := &LogEntry{}
		if err := proto.Unmarshal(v, entry); err != nil {
			logging.Error(err)
			return nil
		}

		logEntries = append(logEntries, entry)
	}

	return logEntries
}

func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 {
		rf.loadRaftState(rf.getRaftBootstrapState())
		return
	}

	r := &RaftState{}
	if err := rf.coder.Decode(data, r); err != nil {
		log.Fatal("raft read persist error")
	}

	rf.loadRaftState(r)
}

func (rf *Raft) changeRole(role Role) {
	logging.Debugf("%s change role: %s -> %s\n", rf.me, rf.role, role)
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

func (rf *Raft) randElectionTimeout() time.Duration {
	r := time.Duration(rand.Int63()) % rf.electionTimeout
	return rf.electionTimeout + r
}

func (rf *Raft) resetElectionTimer() {
	rf.electionTimer.Stop()
	rf.electionTimer.Reset(rf.randElectionTimeout())
}

func (rf *Raft) resetHeartBeatTimers() {
	for name := range rf.appendEntriesTimers {
		rf.appendEntriesTimers[name].Stop()
		rf.appendEntriesTimers[name].Reset(0)
	}
}

func (rf *Raft) resetHeartBeatTimer(name string) {
	rf.appendEntriesTimers[name].Stop()
	rf.appendEntriesTimers[name].Reset(0)
}

func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	close(rf.stopCh)
	close(rf.applyCh)
	close(rf.notifyApplyCh)
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

func (rf *Raft) Put(command []byte) (int32, int32, bool) {
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
		rf.updateCommitIndex()
		rf.persist()
	}
	rf.resetHeartBeatTimers()
	return index, term, isLeader
}

func (rf *Raft) ApplyChan() <-chan ApplyMsg {
	return rf.applyCh
}

func (rf *Raft) startApplyLogs() {
	defer rf.applyTimer.Reset(rf.applyInterval)
	logging.Debugf("%s start apply logs", rf.me)

	rf.mu.Lock()
	var msgs []ApplyMsg
	if rf.lastApplied < rf.lastSnapshotIndex {
		msgs = make([]ApplyMsg, 0, 1)
		msgs = append(msgs, ApplyMsg{
			CommandValid: false,
			Command:      []byte(InstallSnapshotToStore),
			CommandIndex: rf.lastSnapshotIndex - rf.lastApplied,
		})
	} else if rf.commitIndex <= rf.lastApplied {
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

func (rf *Raft) listenApplyMsg() {
	for {
		select {
		case <-rf.stopCh:
			return
		case <-rf.applyTimer.C:
			rf.notifyApplyCh <- struct{}{}
		case <-rf.notifyApplyCh:
			rf.startApplyLogs()
		}
	}
}

func (rf *Raft) listenElection() {
	for {
		select {
		case <-rf.stopCh:
			return
		case <-rf.electionTimer.C:
			rf.startElection()
		}
	}
}

func (rf *Raft) appendEntries() {
	if len(rf.peers) == 0 {

	}

	for name := range rf.peers {
		go rf.appendEntriesToPeer(name)
	}
}

func (rf *Raft) Start(peers map[string]RaftClient) {
	rf.peers = peers

	rf.electionTimer = time.NewTimer(rf.randElectionTimeout())
	rf.appendEntriesTimers = make(map[string]*time.Timer)
	for k := range rf.peers {
		rf.appendEntriesTimers[k] = time.NewTimer(rf.heartbeatTimeout)
	}
	rf.applyTimer = time.NewTimer(rf.applyInterval)
	rf.notifyApplyCh = make(chan struct{}, 100)

	go rf.listenApplyMsg()

	go rf.listenElection()

	rf.appendEntries()
}

func NewRaft(persister *Persister, applyCh chan ApplyMsg, opts *Options) *Raft {
	rf := &Raft{}
	rf.loadOption(opts)
	rf.persister = persister
	rf.applyCh = applyCh
	rf.coder = codec.Get(codec.Gob)

	rf.stopCh = make(chan struct{})
	rf.term = 0
	rf.voteFor = ""
	rf.role = Follower
	rf.logEntries = make([]*LogEntry, 1)
	rf.readPersist(persister.ReadRaftState())

	return rf
}
