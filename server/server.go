package server

import (
	"context"
	"net"
	"time"

	"go.themis.run/themis/config"
	"go.themis.run/themis/logging"
	themis "go.themis.run/themis/pb"
	"go.themis.run/themis/raft"
	"go.themis.run/themis/store"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

type Server struct {
	store store.Store

	raft raft.Server

	info *NodeInfo

	stopch chan struct{}

	infoch chan *raft.RaftInfo
}

type NodeInfo struct {
	Name    string
	Address string
	Term    int32

	LeaderName    string
	LeaderAddress string

	Server map[string]string

	role raft.Role
}

func New(cfg config.Config) *Server {
	info := &NodeInfo{
		Name:    cfg.Name,
		Address: cfg.Address,
		Server:  cfg.ServerAddress,
	}

	logging.DefaultLogger = logging.New(cfg.Log)

	store, err := store.New(cfg.Path, cfg.Size)
	if err != nil {
		panic(err)
	}

	infoch := make(chan *raft.RaftInfo, 1)
	cfg.Raft.InfoCh = infoch

	raft := raft.New(cfg.Raft)

	return &Server{
		store:  store,
		raft:   raft,
		info:   info,
		stopch: make(chan struct{}),
		infoch: infoch,
	}
}

func (s *Server) Run() {
	s.raft.Run()
	go s.listenCommmit()
	go s.listenInfo()

	lis, err := net.Listen("tcp", s.info.Address)
	if err != nil {
		panic(err)
	}

	srv := grpc.NewServer()
	themis.RegisterThemisServer(srv, s)

	if err := srv.Serve(lis); err != nil {
		panic(err)
	}
}

func (s *Server) Put(ctx context.Context, req *themis.PutRequest) (*themis.PutResponse, error) {
	reply := &themis.PutResponse{
		Header: s.newHeader(),
	}

	select {
	case <-ctx.Done():
		return reply, ctx.Err()
	default:
	}

	if s.info.role != raft.Leader {
		reply.Header.Success = false
		return reply, nil
	}

	commend := &themis.Command{
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
		Header: s.newHeader(),
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
		Header: s.newHeader(),
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if s.info.role != raft.Leader {
		reply.Header.Success = false
		return reply, nil
	}

	commend := &themis.Command{
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
	watcher := s.store.Watch(req.Key, req.Type.String(), false)

	for {
		select {
		case <-s.stopch:
			break
		case event := <-watcher.EventChan():
			reply := &themis.WatchResponse{
				Header: s.newHeader(),
			}

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

			if err := tw.Send(reply); err != nil {
				return err
			}
		}
	}
}

func (s *Server) Watch(ctx context.Context, req *themis.WatchRequest) (*themis.WatchResponse, error) {
	reply := &themis.WatchResponse{
		Header: s.newHeader(),
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

func (s *Server) SearchByPrefix(ctx context.Context, req *themis.SearchRequest) (*themis.SearchResponse, error) {
	reply := &themis.SearchResponse{
		Header: s.newHeader(),
	}

	select {
	case <-ctx.Done():
		return reply, ctx.Err()
	default:
	}

	nodeList := s.store.ListNodeByPrefix(req.PrefixKey)
	kvList := make([]*themis.KV, 0)

	for _, node := range nodeList {
		kv := &themis.KV{
			Key:        node.Key,
			Value:      node.Value,
			CreateTime: node.CreateTime.UnixMilli(),
			Ttl:        node.TTL.Milliseconds(),
		}

		kvList = append(kvList, kv)
	}

	reply.KvList = kvList

	return reply, nil
}

func (s *Server) listenCommmit() {
	for {
		select {
		case <-s.stopch:
			break
		case commendBuffer := <-s.raft.CommitChannel():
			commend := &themis.Command{}
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

func (s *Server) listenInfo() {
	for {
		select {
		case <-s.stopch:
			break
		case info := <-s.infoch:
			s.info.role = info.Role
			s.info.LeaderName = info.Leader
			s.info.LeaderAddress = s.info.Server[info.Leader]
			s.info.Term = info.Term
		}
	}
}

func (s *Server) newHeader() *themis.Header {
	return &themis.Header{
		MemberName:    s.info.Name,
		MemberAddress: s.info.Address,
		Term:          s.info.Term,
		LeaderName:    s.info.LeaderName,
		LeaderAddress: s.info.LeaderAddress,
		Role:          string(s.info.role),
		Servers:       s.info.Server,
		Success:       false,
	}
}
