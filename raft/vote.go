package raft

import (
	"context"
	"time"

	"go.themis.run/themis/logging"
)

func (rf *Raft) Vote(ctx context.Context, req *VoteRequest) (reply *VoteReply, err error) {
	reply = &VoteReply{}
	rf.mu.Lock()
	defer rf.mu.Unlock()

	lastLogTerm, lastLogIndex := rf.lastLogTermIndex()
	reply.Term = int32(rf.term)
	reply.VoteGranted = false
	reply.Base = &RaftBase{
		From: req.Base.To,
		To:   req.Base.From,
	}

	if req.Term < rf.term {
		return
	} else if req.Term == rf.term {
		if rf.role == Leader {
			return
		}
		if rf.voteFor == req.CandidateName {
			reply.VoteGranted = true
		}
		if rf.voteFor != "" && rf.voteFor != req.CandidateName {
			return
		}
	}

	defer rf.persist()

	if req.Term > rf.term {
		rf.term = req.Term
		rf.voteFor = ""
		rf.changeRole(Follower)
	}

	if lastLogTerm > req.LastLogTerm || (req.LastLogTerm == lastLogTerm && req.LastLogIndex < int32(lastLogIndex)) {
		// log term and index are not up to date
		return reply, nil
	}

	rf.term = req.Term
	rf.voteFor = req.CandidateName
	reply.VoteGranted = true
	rf.changeRole(Follower)
	rf.resetElectionTimer()
	logging.Debugf("%s vote to %s\n", rf.me, req.CandidateName)
	return
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
			req.Base = &RaftBase{
				From: rf.me,
				To:   name,
			}

			ctx, cancel := context.WithTimeout(context.Background(), RPCTimeout)
			defer cancel()

			t := time.Now()
			resp, err := rf.peers[name].Vote(ctx, req)
			logging.Debugf("%s -> %s vote request time: %d ms\n", rf.me, name, time.Now().Sub(t)/time.Millisecond)
			if err != nil {
				logging.Debugf("%s -> %s error: ", rf.me, name)
				logging.Debug(err)
				return
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
		logging.Debugf("%s elect fail. get tickets num: %d\n", rf.me, grantedCount)
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
