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
	Name        string
	Address     string
	RaftAddress string
	Term        int32

	LeaderName    string
	LeaderAddress string // raft address

	Peers map[string]config.Address

	role raft.Role
}

func New(cfg config.Config) *Server {
	info := &NodeInfo{
		Name:        cfg.Name,
		Address:     cfg.Address,
		RaftAddress: cfg.Raft.Address,
		Peers:       cfg.PeerAddress,
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
			s.info.LeaderAddress = s.info.Peers[info.Leader].RaftAddress
			s.info.Term = info.Term
		}
	}
}
