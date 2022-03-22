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
		// 尝试跳过一个 term
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
