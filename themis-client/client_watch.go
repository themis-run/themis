package themisclient

import (
	"context"
	"io"

	"go.themis.run/themis/logging"
	themis "go.themis.run/themis/pb"
)

type OperateType int32

const (
	ALL    OperateType = 0
	Set    OperateType = 1
	Get    OperateType = 2
	Delete OperateType = 3
	Write  OperateType = 4
	Expire OperateType = 5
)

type WatchCallback func(preKV, kv *themis.KV, t OperateType) error

func (c *Client) Watch(key string, op OperateType, callback WatchCallback) error {
	addr := c.balancer.Get(c.info.LeaderName, c.info.Servers, false)
	tclient, err := c.newClient(addr)
	if err != nil {
		return err
	}

	req := &themis.WatchRequest{
		Key:  key,
		Type: themis.OperateType(op),
	}

	go func() {
		resp, err := tclient.Watch(context.Background(), req)
		if err != nil {
			logging.Error(err)
			return
		}

		if err := callback(resp.PrevKv, resp.Kv, OperateType(resp.GetType())); err != nil {
			logging.Error(err)
		}
	}()

	return nil
}

func (c *Client) WatchStream(key string, op OperateType, callback WatchCallback) error {
	addr := c.balancer.Get(c.info.LeaderName, c.info.Servers, false)
	tclient, err := c.newClient(addr)
	if err != nil {
		return err
	}

	req := &themis.WatchRequest{
		Key:  key,
		Type: themis.OperateType(op),
	}
	stream, err := tclient.WatchStream(context.Background(), req)
	if err != nil {
		return err
	}

	go func() {
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				logging.Error(err)
			}

			if err := callback(resp.PrevKv, resp.Kv, OperateType(resp.GetType())); err != nil {
				logging.Error(err)
			}
		}
	}()

	return nil
}
