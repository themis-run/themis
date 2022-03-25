package raft

import (
	"context"
	"time"

	"go.themis.run/themis/logging"
)

func (rf *Raft) InstallSnapshot(ctx context.Context, req *InstallSnapshotRequest) (reply *InstallSnapshotReply, err error) {
	reply = &InstallSnapshotReply{}
	rf.mu.Lock()
	defer rf.mu.Unlock()

	reply.Term = rf.term
	if req.Term < rf.term {
		return
	}

	if req.Term > rf.term || rf.role != Follower {
		rf.term = req.Term
		rf.changeRole(Follower)
		rf.resetElectionTimer()
		defer rf.persist()
	}

	if rf.lastSnapshotIndex >= req.LastIncludedIndex {
		return
	}

	start := req.LastIncludedIndex - rf.lastSnapshotIndex
	if start >= int32(len(rf.logEntries)) {
		rf.logEntries = make([]*LogEntry, 1)
		rf.logEntries[0].Term = req.LastIncludedTerm
		rf.logEntries[0].Index = req.LastIncludedIndex
	} else {
		rf.logEntries = rf.logEntries[start:]
	}

	rf.lastSnapshotIndex = req.LastIncludedIndex
	rf.lastSnapshotTerm = req.LastIncludedTerm

	var state []byte
	state, err = rf.getPersistData()
	if err != nil {
		logging.Debugf("%s get persist data error ", rf.me)
		logging.Debug(err)
		return
	}

	rf.persister.SaveStateAndSnapshot(state, req.Data)
	return
}

func (rf *Raft) doInstallSnapshot(peerName string, req *InstallSnapshotRequest, reply *InstallSnapshotReply) (err error) {
	t := time.Now()
	defer logging.Debugf("%s -> %s vote request time: %d ms\n", rf.me, peerName, time.Now().Sub(t)/time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), rf.rpcTimeout)
	defer cancel()

	reply, err = rf.peers[peerName].InstallSnapshot(ctx, req)
	return err
}

func (rf *Raft) sendInstallSnapshot(peerName string) {
	rf.mu.Lock()
	req := &InstallSnapshotRequest{
		Term:              rf.term,
		LeaderName:        rf.me,
		LastIncludedIndex: rf.lastSnapshotIndex,
		LastIncludedTerm:  rf.lastSnapshotTerm,
		Data:              rf.persister.ReadSnapshot(),
	}
	rf.mu.Unlock()

	for !rf.killed() {
		reply := &InstallSnapshotReply{}

		if err := rf.doInstallSnapshot(peerName, req, reply); err != nil {
			// avoid full CPU
			logging.Debugf("%s -> %s do install snapshot error ", rf.me, peerName)
			logging.Debug(err)
			time.Sleep(10 * time.Millisecond)
			continue
		}

		rf.mu.Lock()
		if rf.term != req.Term || rf.role != Leader {
			return
		}

		if reply.Term > rf.term {
			rf.changeRole(Follower)
			rf.resetElectionTimer()
			rf.term = reply.Term
			rf.persist()
			return
		}

		if req.LastIncludedIndex > rf.matchIndex[peerName] {
			rf.matchIndex[peerName] = req.LastIncludedIndex
		}

		if req.LastIncludedIndex+1 > rf.nextIndex[peerName] {
			rf.nextIndex[peerName] = req.LastIncludedIndex + 1
		}
		rf.mu.Unlock()
		return
	}
}
