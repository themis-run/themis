package loadbalance

import (
	"math/rand"
	"time"

	themisclient "go.themis.run/themis/themis-client"
)

type LoadBalancer interface {
	Get(themisclient.Info, bool) string
}

func New(name string) LoadBalancer {
	return &loadbalancer{}
}

type loadbalancer struct {
}

func (l *loadbalancer) Get(info themisclient.Info, isWrite bool) string {
	if isWrite {
		return info.Servers[info.LeaderName]
	}

	rand.Seed(time.Now().Unix())
	randomNum := rand.Intn(len(info.Servers))

	for _, v := range info.Servers {
		if randomNum == 0 {
			return v
		}
		randomNum--
	}

	return info.Servers[info.LeaderName]
}
