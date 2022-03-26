package server

import (
	"context"
	"time"

	"go.themis.run/themis/logging"
	themis "go.themis.run/themis/pb"
	"go.themis.run/themis/raft"
	"go.themis.run/themis/store"

	"google.golang.org/protobuf/proto"
)

type Server struct {
	store store.Store

	raft raft.Server

	info *NodeInfo

	stopch chan struct{}
}

type NodeInfo struct {
	Name    string
	Address string

	LeaderName    string
	LeaderAddress string

	role raft.Role
}

func (s *Server) Put(ctx context.Context, req *themis.PutRequest) (*themis.PutResponse, error) {
	reply := &themis.PutResponse{
		Header: &themis.Header{
			MemberName:    s.info.Name,
			MemberAddress: s.info.Address,
			LeaderName:    s.info.LeaderName,
			LeaderAddress: s.info.LeaderAddress,
			Role:          string(s.info.role),
			Success:       false,
		},
	}

	select {
	case <-ctx.Done():
		return reply, ctx.Err()
	default:
	}

	commend := &themis.Commend{
		Type: themis.OperateType_Set,
		Kv:   req.Kv,
	}
	commendBuffer, err := proto.Marshal(commend)
	if err != nil {
		return reply, err
	}

	reply.Header.Success = s.raft.Put(commendBuffer)

	return reply, nil
}

func (s *Server) Get(ctx context.Context, req *themis.GetRequest) (*themis.GetResponse, error) {
	reply := &themis.GetResponse{
		Header: &themis.Header{
			MemberName:    s.info.Name,
			MemberAddress: s.info.Address,
			LeaderName:    s.info.LeaderName,
			LeaderAddress: s.info.LeaderAddress,
			Role:          string(s.info.role),
			Success:       false,
		},
	}

	select {
	case <-ctx.Done():
		return reply, ctx.Err()
	default:
	}

	event := s.store.Get(req.Key)
	reply.Kv = &themis.KV{
		Key:        event.Node.Key,
		Value:      event.Node.Value,
		CreateTime: event.Node.CreateTime.UnixMilli(),
		Ttl:        event.Node.TTL.Milliseconds(),
	}

	reply.Header.Success = true

	return reply, nil
}

func (s *Server) Delete(ctx context.Context, req *themis.DeleteRequest) (*themis.DeleteResponse, error) {
	reply := &themis.DeleteResponse{
		Header: &themis.Header{
			MemberName:    s.info.Name,
			MemberAddress: s.info.Address,
			LeaderName:    s.info.LeaderName,
			LeaderAddress: s.info.LeaderAddress,
			Role:          string(s.info.role),
			Success:       false,
		},
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	commend := &themis.Commend{
		Type: themis.OperateType_Delete,
		Kv: &themis.KV{
			Key: req.Key,
		},
	}
	commendBuffer, err := proto.Marshal(commend)
	if err != nil {
		return reply, err
	}

	reply.Header.Success = s.raft.Put(commendBuffer)

	return reply, nil
}

func (s *Server) WatchStream(req *themis.WatchRequest, tw themis.Themis_WatchStreamServer) error {
	return nil
}

func (s *Server) Watch(ctx context.Context, req *themis.WatchRequest) (*themis.WatchResponse, error) {
	reply := &themis.WatchResponse{
		Header: &themis.Header{
			MemberName:    s.info.Name,
			MemberAddress: s.info.Address,
			LeaderName:    s.info.LeaderName,
			LeaderAddress: s.info.LeaderAddress,
			Role:          string(s.info.role),
			Success:       false,
		},
	}

	select {
	case <-ctx.Done():
		return reply, ctx.Err()
	default:
	}

	watcher := s.store.Watch(req.Key, req.Type.String(), false)

	event := <-watcher.EventChan()

	reply.Kv = &themis.KV{
		Key:        event.Node.Key,
		Value:      event.Node.Value,
		CreateTime: event.Node.CreateTime.UnixMilli(),
		Ttl:        event.Node.TTL.Milliseconds(),
	}

	reply.PrevKv = &themis.KV{
		Key:        event.OldNode.Key,
		Value:      event.OldNode.Value,
		CreateTime: event.OldNode.CreateTime.UnixMilli(),
		Ttl:        event.OldNode.TTL.Milliseconds(),
	}

	return reply, nil
}

func (s *Server) listenCommmit() {
	for {
		select {
		case <-s.stopch:
			break
		case commendBuffer := <-s.raft.CommitChannel():
			commend := &themis.Commend{}
			err := proto.Unmarshal(commendBuffer, commend)
			if err != nil {
				logging.Error(err)
			}

			switch commend.Type {
			case themis.OperateType_Set:
				s.store.Set(commend.Kv.Key, commend.Kv.Value, time.Duration(commend.Kv.Ttl))
			case themis.OperateType_Delete:
				s.store.Delete(commend.Kv.Key)
			}
		}
	}
}
