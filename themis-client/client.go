package themisclient

import (
	"context"
	"sync"
	"time"

	"go.themis.run/themis/codec"
	"go.themis.run/themis/logging"
	themis "go.themis.run/themis/pb"
	"go.themis.run/themis/themis-client/loadbalance"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	config   *Config
	balancer loadbalance.LoadBalancer
	info     *Info

	coder   codec.Codec
	clients map[string]themis.ThemisClient

	sync.Mutex
}

type Info struct {
	LeaderName string
	Term       int32
	Servers    map[string]string
}

func NewClient(config *Config) *Client {
	balancer := loadbalance.New(config.LoadBalancerName)
	return &Client{
		balancer: balancer,
	}
}

func (c *Client) Get(key string) (*themis.KV, error) {
	addr := c.balancer.Get(c.info.LeaderName, c.info.Servers, false)
	tclient, err := c.newClient(addr)
	if err != nil {
		return nil, err
	}

	req := &themis.GetRequest{
		Key: key,
	}

	resp, err := tclient.Get(context.Background(), req)
	if err != nil {
		return nil, err
	}

	c.updateInfo(resp.GetHeader())

	return resp.GetKv(), nil
}

func (c *Client) Delete(key string) error {
	var isRetry bool
	var err error

	for i := 0; i < c.config.RetryNum; i++ {
		addr := c.balancer.Get(c.info.LeaderName, c.info.Servers, true)
		isRetry, err = c.delete(addr, key)
		if err != nil {
			logging.Error(err)
		}

		if !isRetry {
			break
		}
	}

	return nil
}

func (c *Client) Set(key string, value interface{}) error {
	return c.SetWithExpireTime(key, value, 0)
}

func (c *Client) SetWithExpireTime(key string, value interface{}, ttl time.Duration) error {
	var isRetry bool
	var err error

	for i := 0; i < c.config.RetryNum; i++ {
		addr := c.balancer.Get(c.info.LeaderName, c.info.Servers, true)
		isRetry, err = c.put(addr, key, value, ttl)
		if err != nil {
			logging.Error(err)
		}

		if !isRetry {
			break
		}
	}

	return err
}

func (c *Client) newClient(address string) (themis.ThemisClient, error) {
	if client, ok := c.clients[address]; ok {
		return client, nil
	}

	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	client := themis.NewThemisClient(conn)

	c.clients[address] = client

	return client, nil
}

func (c *Client) put(address, key string, value interface{}, ttl time.Duration) (bool, error) {
	tclient, err := c.newClient(address)
	if err != nil {
		return true, err
	}

	bytes, err := c.coder.Encode(value)
	if err != nil {
		return true, err
	}

	req := &themis.PutRequest{
		Kv: &themis.KV{
			Key:        key,
			Value:      bytes,
			CreateTime: time.Now().UnixMilli(),
			Ttl:        ttl.Milliseconds(),
		},
	}

	resp, err := tclient.Put(context.Background(), req)
	if err != nil {
		return false, err
	}

	c.updateInfo(resp.GetHeader())

	return !resp.Header.Success, nil
}

func (c *Client) delete(address, key string) (bool, error) {
	tclient, err := c.newClient(address)
	if err != nil {
		return true, err
	}

	req := &themis.DeleteRequest{
		Key: key,
	}

	resp, err := tclient.Delete(context.Background(), req)
	if err != nil {
		return false, err
	}

	c.updateInfo(resp.Header)

	return !resp.Header.Success, nil
}

func (c *Client) updateInfo(header *themis.Header) {
	c.Lock()
	defer c.Unlock()

	if header.Term > c.info.Term {
		c.info.LeaderName = header.LeaderName
		c.info.Servers = header.Servers
		c.info.Term = header.Term
	}
}
