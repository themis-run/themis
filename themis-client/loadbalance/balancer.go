package loadbalance

import (
	"math/rand"
	"time"
)

func init() {
	rand.Seed(time.Now().Unix())
	Register(defaultName, &loadbalancer{})
}

type LoadBalancer interface {
	Get(string, map[string]string, bool) string
}

var loadbalancerMap map[string]LoadBalancer
var defaultName = "default"

func New(name string) LoadBalancer {
	if v, ok := loadbalancerMap[name]; ok {
		return v
	}
	return &loadbalancer{}
}

func Register(name string, loadbalancer LoadBalancer) {
	if loadbalancerMap == nil {
		loadbalancerMap = make(map[string]LoadBalancer)
	}

	loadbalancerMap[name] = loadbalancer
}

type loadbalancer struct {
}

func (l *loadbalancer) Get(leaderName string, servers map[string]string, isWrite bool) string {
	if isWrite {
		return servers[leaderName]
	}

	randomNum := rand.Intn(len(servers))

	for _, v := range servers {
		if randomNum == 0 {
			return v
		}
		randomNum--
	}

	return servers[leaderName]
}
