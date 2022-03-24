package raft

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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

	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultServiceConfig(retryPolicy))
	if err != nil {
		return nil, err
	}

	return NewRaftClient(conn), nil
}
