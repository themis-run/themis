package loadbalance

import (
	"math/rand"
	"time"
)

type LoadBalancer interface {
	Get(string, map[string]string, bool) string
}

func New(name string) LoadBalancer {
	return &loadbalancer{}
}

type loadbalancer struct {
}

func (l *loadbalancer) Get(leaderName string, servers map[string]string, isWrite bool) string {
	if isWrite {
		return servers[leaderName]
	}

	rand.Seed(time.Now().Unix())
	randomNum := rand.Intn(len(servers))

	for _, v := range servers {
		if randomNum == 0 {
			return v
		}
		randomNum--
	}

	return servers[leaderName]
}
