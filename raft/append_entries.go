package raft

import (
	context "context"
)

func (rf *Raft) AppendEntries(ctx context.Context, req *AppendEntriesRequest) (reply *AppendEntriesReply, err error) {
	rf.mu.Lock()

	reply.Term = rf.term

	if rf.term > req.Term {
		rf.mu.Unlock()
		return
	}

	rf.term = req.Term
	rf.changeRole(Follower)
	rf.resetElectionTimer()
	_, lastLogIndex := rf.lastLogTermIndex()

	if req.PrevLogIndex < int32(rf.lastSnapshotIndex) {
		reply.Success = false
		reply.NextIndex = int32(rf.lastSnapshotIndex) + 1
	} else if req.PrevLogIndex > lastLogIndex {
		reply.Success = false
		reply.NextIndex = rf.getNextIndex()
	} else if req.PrevLogIndex == rf.lastSnapshotIndex {
		if rf.outOfOrderAppendEntries(req) {
			reply.Success = false
			reply.NextIndex = 0
		} else {
			reply.Success = true
			rf.logEntries = append(rf.logEntries[:1], req.Entries...) // 保留 logs[0]
			reply.NextIndex = rf.getNextIndex()
		}
	} else if rf.logEntries[rf.getRealIdxByLogIndex(req.PrevLogIndex)].Term == req.PrevLogTerm {
		if rf.outOfOrderAppendEntries(req) {
			reply.Success = false
			reply.NextIndex = 0
		} else {
			reply.Success = true
			rf.logEntries = append(rf.logEntries[0:rf.getRealIdxByLogIndex(req.PrevLogIndex)+1], req.Entries...)
			reply.NextIndex = rf.getNextIndex()
		}
	} else {
		reply.Success = false
		term := rf.logEntries[rf.getRealIdxByLogIndex(req.PrevLogIndex)].Term
		idx := req.PrevLogIndex
		for idx > rf.commitIndex && idx > rf.lastSnapshotIndex && rf.logEntries[rf.getRealIdxByLogIndex(idx)].Term == term {
			idx -= 1
		}
		reply.NextIndex = idx + 1
	}

	if reply.Success {
		if rf.commitIndex < req.LeaderCommit {
			rf.commitIndex = req.LeaderCommit
			rf.notifyApplyCh <- struct{}{}
		}
	}

	rf.persist()
	rf.mu.Unlock()

	return
}

func (rf *Raft) outOfOrderAppendEntries(args *AppendEntriesRequest) bool {
	argsLastIndex := args.PrevLogIndex + int32(len(args.Entries))
	lastTerm, lastIndex := rf.lastLogTermIndex()
	if argsLastIndex < lastIndex && lastTerm == args.Term {
		return true
	}
	return false
}

func (rf *Raft) getNextIndex() int32 {
	_, idx := rf.lastLogTermIndex()
	return idx + 1
}

func (rf *Raft) appendEntries() {
	for name := range rf.peers {
		go func(name string) {
			for {
				select {
				case <-rf.stopCh:
					rf.Kill()
				case <-rf.appendEntriesTimers[name].C:

				}
			}
		}(name)
	}
}

func (rf *Raft) appendEntriesToPeer(name string) {
	for {
		select {
		case <-rf.stopCh:
			rf.Kill()
		case <-rf.appendEntriesTimers[name].C:
			rf.sendAppendEntriesRPCToPeer(name)
		}
	}
}

func (rf *Raft) sendAppendEntriesRPCToPeer(name string) {
	for !rf.killed() {
		rf.mu.Lock()
		if rf.role != Leader {
			rf.resetHeartBeatTimer(name)
			rf.mu.Unlock()
			return
		}

		req := rf.getAppendEntriesRequst(name)
		rf.resetHeartBeatTimer(name)
		rf.mu.Unlock()

		reply := &AppendEntriesReply{}
		if err := rf.doAppendLogsToPeer(name, req, reply); err != nil {
			continue
		}

		if isRetry := rf.processAppendLogsReply(name, req, reply); isRetry {
			continue
		}
		return
	}
}

func (rf *Raft) processAppendLogsReply(peerName string, req *AppendEntriesRequest, reply *AppendEntriesReply) bool {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if reply.Term > rf.term {
		rf.changeRole(Follower)
		rf.resetElectionTimer()
		rf.term = reply.Term
		rf.persist()
		return false
	}

	if rf.role != Leader || rf.term != req.Term {
		return false
	}

	if reply.Success {
		if reply.NextIndex > rf.nextIndex[peerName] {
			rf.nextIndex[peerName] = reply.NextIndex
			rf.matchIndex[peerName] = reply.NextIndex - 1
		}

		if len(req.Entries) > 0 && req.Entries[len(req.Entries)-1].Term == rf.term {
			rf.updateCommitIndex()
		}

		rf.persist()
		return false
	}

	if reply.NextIndex > rf.lastSnapshotIndex {
		rf.nextIndex[peerName] = reply.NextIndex
		return true
	}

	go rf.sendInstallSnapshot(peerName)
	return false
}

func (rf *Raft) updateCommitIndex() {
	hasCommit := false
	for i := rf.commitIndex + 1; i <= rf.lastSnapshotIndex+int32(len(rf.logEntries)); i++ {
		count := 0
		for _, m := range rf.matchIndex {
			if m >= i {
				count += 1
				if count > len(rf.peers)/2 {
					rf.commitIndex = i
					hasCommit = true
					break
				}
			}
		}
		if rf.commitIndex != i {
			break
		}
	}
	if hasCommit {
		rf.notifyApplyCh <- struct{}{}
	}
}

func (rf *Raft) doAppendLogsToPeer(name string, req *AppendEntriesRequest, resp *AppendEntriesReply) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), RPCTimeout)
	defer cancel()

	resp, err = rf.peers[name].AppendEntries(ctx, req)
	if err != nil {
		//log
	}
	return err
}

func (rf *Raft) getAppendLogs(name string) (prevLogIndex, prevLogTerm int32, entries []*LogEntry) {
	nextIndex := rf.nextIndex[name]
	lastLogTerm, lastLogIndex := rf.lastLogTermIndex()
	if nextIndex <= rf.lastSnapshotIndex || nextIndex > lastLogIndex {
		prevLogIndex = lastLogIndex
		prevLogTerm = lastLogTerm
		return
	}

	entries = append([]*LogEntry{}, rf.logEntries[rf.getRealIdxByLogIndex(nextIndex):]...)
	prevLogIndex = nextIndex - 1

	if prevLogIndex == rf.lastSnapshotIndex {
		prevLogTerm = rf.lastSnapshotTerm
		return
	}

	prevLogTerm = rf.getLogByIndex(prevLogIndex).Term

	return
}

func (rf *Raft) getAppendEntriesRequst(name string) *AppendEntriesRequest {
	prevLogIndex, prevLogTerm, logs := rf.getAppendLogs(name)
	return &AppendEntriesRequest{
		Term:         rf.term,
		LeaderName:   rf.me,
		PrevLogIndex: prevLogIndex,
		PrevLogTerm:  prevLogTerm,
		Entries:      logs,
		LeaderCommit: rf.commitIndex,
	}
}
