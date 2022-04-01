package themisclient

import (
	"fmt"
	"time"
)

type Instance struct {
	ServiceName string
	ClusterName string
	IP          string
	Port        int
	IsHealthy   bool
	TTL         time.Duration
	MetaData    []byte
}

const (
	ServiceMark        = "service"
	DefaultClusterName = "/"
)

func (c *Client) RegisterInstanceWithIP(serviceName, ip string, port int) error {
	return c.RegisterInstanceWithIPCluster(DefaultClusterName, serviceName, ip, port)
}

func (c *Client) RegisterInstanceWithIPCluster(serviceName, clusterName, ip string, port int) error {
	instance := &Instance{
		ClusterName: clusterName,
		ServiceName: serviceName,
		IP:          ip,
		Port:        port,
	}
	return c.RegisterInstance(instance)
}

func (c *Client) RegisterInstance(instance *Instance) error {
	key := convertInstanceKey(instance.ServiceName, instance.ClusterName, instance.IP)
	return c.SetWithExpireTime(key, instance, instance.TTL)
}

func (c *Client) RegisterInstanceWithHeartbeat(instance *Instance, heartbeatTimeout time.Duration) error {
	if err := c.RegisterInstance(instance); err != nil {
		return err
	}

	go func() {
		c.Heartbeat(instance)
		time.Sleep(heartbeatTimeout)
	}()

	return nil
}

func (c *Client) DiscoveryService(serviceName string) ([]*Instance, error) {
	prefix := fmt.Sprintf("%s:%s", ServiceMark, serviceName)

	kvList, err := c.SearchByPrefix(prefix)
	if err != nil {
		return nil, err
	}

	instanceList := make([]*Instance, 0)
	for _, kv := range kvList {
		instance := &Instance{}
		if err := c.coder.Decode(kv.Value, instance); err != nil {
			return nil, err
		}

		instance.IsHealthy = getInstanceHealthy(kv.CreateTime, kv.Ttl)

		instanceList = append(instanceList, instance)
	}

	return instanceList, nil
}

func (c *Client) Heartbeat(instance *Instance) error {
	key := convertInstanceKey(instance.ServiceName, instance.ClusterName, instance.IP)
	return c.SetWithExpireTime(key, instance, instance.TTL)
}

func convertInstanceKey(serviceName, clusterName, ip string) string {
	if clusterName == "" {
		clusterName = DefaultClusterName
	}

	return fmt.Sprintf("%s:%s:%s:%s", ServiceMark, serviceName, clusterName, ip)
}

func getInstanceHealthy(createTime, ttl int64) bool {
	expiretime := time.UnixMilli(createTime + ttl)
	return time.Now().Before(expiretime)
}
