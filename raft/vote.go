package raft

import (
	"context"
	"errors"
)

var ErrorVote = errors.New("vote error")

func (rf *Raft) Vote(ctx context.Context, req *VoteRequest) (*VoteReply, error) {
	reply := &VoteReply{}
	rf.mu.Lock()
	defer rf.mu.Unlock()

	lastLogTerm, lastLogIndex := rf.lastLogTermIndex()
	reply.Term = int32(rf.term)
	reply.VoteGranted = false

	if req.Term < rf.term {
		return reply, nil
	} else if req.Term == rf.term {
		if rf.role == Leader {
			return reply, nil
		}
		if rf.voteFor == req.CandidateName {
			reply.VoteGranted = true
		}
		if rf.voteFor != "" && rf.voteFor != req.CandidateName {
			return reply, nil
		}
	}

	defer rf.persist()

	if req.Term > rf.term {
		rf.term = req.Term
		rf.voteFor = ""
		rf.changeRole(Follower)
	}

	if lastLogTerm > req.LastLogTerm || (req.LastLogTerm == lastLogTerm && req.LastLogIndex < int32(lastLogIndex)) {
		// 选取限制
		return reply, nil
	}

	rf.term = req.Term
	rf.voteFor = req.CandidateName
	reply.VoteGranted = true
	rf.changeRole(Follower)
	rf.resetElectionTimer()
	return nil, ErrorVote
}

func (rf *Raft) startElection() {
	rf.mu.Lock()
	rf.electionTimer.Reset(randElectionTimeout())
	if rf.role == Leader {
		rf.mu.Unlock()
		return
	}

	rf.changeRole(Candidate)
	lastTerm, lastLogIndex := rf.lastLogTermIndex()
	req := &VoteRequest{
		Term:          rf.term,
		CandidateName: rf.me,
		LastLogIndex:  int32(lastLogIndex),
		LastLogTerm:   lastTerm,
	}
	rf.persist()
	rf.mu.Unlock()

	grantedCount := 1
	count := 1
	ticketsCh := make(chan bool, len(rf.peers))
	for name := range rf.peers {
		go func(ch chan bool, name string) {
			ctx, cancel := context.WithTimeout(context.Background(), RPCTimeout)
			defer cancel()

			resp, err := rf.peers[name].Vote(ctx, req)
			if err != nil {
				// log
			}

			ticketsCh <- resp.VoteGranted
			if resp.Term > req.Term {
				rf.mu.Lock()
				if rf.term < resp.Term {
					rf.term = resp.Term
					rf.changeRole(Follower)
					rf.resetElectionTimer()
					rf.persist()
				}
				rf.mu.Unlock()
			}
		}(ticketsCh, name)
	}

	for {
		vote := <-ticketsCh
		count += 1
		if vote {
			grantedCount += 1
		}
		if count == len(rf.peers) || grantedCount > len(rf.peers)/2 || count-grantedCount > len(rf.peers)/2 {
			break
		}
	}

	if grantedCount <= len(rf.peers)/2 {
		// election fail
		return
	}

	rf.mu.Lock()
	if rf.term == req.Term && rf.role == Candidate {
		rf.changeRole(Leader)
		rf.persist()
	}

	if rf.role == Leader {
		rf.resetHeartBeatTimers()
	}
	rf.mu.Unlock()
}
