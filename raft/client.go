package raft

import (
	"google.golang.org/grpc"
)

func newClient(address string) (RaftClient, error) {
	retryPolicy := `{
		"methodConfig": [{
		  "name": [{"service": "raft.Raft"}],
		  "retryPolicy": {
			  "MaxAttempts": 4,
			  "InitialBackoff": ".01s",
			  "MaxBackoff": ".01s",
			  "BackoffMultiplier": 1.0,
			  "RetryableStatusCodes": [ "UNAVAILABLE" ]
		  }
		}]}`

	conn, err := grpc.Dial(address, grpc.WithDefaultServiceConfig(retryPolicy))
	if err != nil {
		return nil, err
	}

	return NewRaftClient(conn), nil
}
