package themisclient

import (
	"fmt"
	"time"
)

type Service struct {
	ClusterName string
	ServiceName string
	IP          string
	Port        int
	TTL         time.Duration
	MetaData    []byte
}

const (
	ServiceMark        = "service"
	DefaultClusterName = "/"
)

func (c *Client) RegisterService(s *Service) error {
	key := convertServiceKey(s.ClusterName, s.ServiceName)
	return c.SetWithExpireTime(key, s, s.TTL)
}

func (c *Client) DeleteService(serviceName string) error {
	return c.DeleteServiceWithCluster(DefaultClusterName, serviceName)
}

func (c *Client) DeleteServiceWithCluster(clusterName, serviceName string) error {
	key := convertServiceKey(clusterName, serviceName)
	return c.Delete(key)
}

func (c *Client) DiscoverService(serviceName string) (*Service, error) {
	return c.DiscoverServiceWithCluster(DefaultClusterName, serviceName)
}

func (c *Client) DiscoverServiceWithCluster(clusterName, serviceName string) (*Service, error) {
	key := convertServiceKey(clusterName, serviceName)

	kv, err := c.Get(key)
	if err != nil {
		return nil, err
	}

	s := &Service{}
	if err := c.coder.Decode(kv.Value, s); err != nil {
		return nil, err
	}

	return s, err
}

func (c *Client) RegisterServiceWithHeartbeat(s *Service, heartbeatTimeout time.Duration) error {
	if err := c.RegisterService(s); err != nil {
		return err
	}

	go func() {
		for {
			c.Heartbeat(s)
			time.Sleep(heartbeatTimeout)
		}
	}()

	return nil
}

func (c *Client) Heartbeat(s *Service) error {
	key := convertServiceKey(s.ClusterName, s.ServiceName)
	return c.SetWithExpireTime(key, s, s.TTL)
}

func (c *Client) WatchServiceStream(serviceName string, op OperateType, callback WatchCallback) error {
	return c.WatchServiceStreamWithCluster(DefaultClusterName, serviceName, op, callback)
}

func (c *Client) WatchServiceStreamWithCluster(clusterName, serviceName string, op OperateType, callback WatchCallback) error {
	key := convertServiceKey(clusterName, serviceName)
	return c.WatchStream(key, op, callback)
}

func (c *Client) WatchService(serviceName string, op OperateType, callback WatchCallback) error {
	return c.WatchServiceWithCluster(DefaultClusterName, serviceName, op, callback)
}

func (c *Client) WatchServiceWithCluster(clusterName, serviceName string, op OperateType, callback WatchCallback) error {
	key := convertServiceKey(clusterName, serviceName)
	return c.Watch(key, op, callback)
}

func convertServiceKey(clusterName, serviceName string) string {
	if clusterName == "" {
		clusterName = DefaultClusterName
	}

	return fmt.Sprintf("%s:%s:%s", ServiceMark, clusterName, serviceName)
}
