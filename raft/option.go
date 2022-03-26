package raft

import (
	"time"

	"go.themis.run/themis/codec"
)

type Options struct {
	NativeName       string
	CodecType        string
	ApplyMsgLength   int
	ElectionTimeout  time.Duration
	HeartBeatTimeout time.Duration
	ApplyInterval    time.Duration
	RPCTimeout       time.Duration
	RaftPeers        map[string]string
}

func DefaultOptions() *Options {
	return &Options{
		CodecType:        codec.Gob,
		ApplyMsgLength:   10,
		ElectionTimeout:  900 * time.Millisecond,
		HeartBeatTimeout: 450 * time.Millisecond,
		ApplyInterval:    300 * time.Millisecond,
		RPCTimeout:       300 * time.Millisecond,
	}
}

type Option func(*Options)

func WithNativeName(name string) Option {
	return func(o *Options) {
		o.NativeName = name
	}
}

func WithRaftClient(name, addr string) Option {
	return func(o *Options) {
		if o.RaftPeers == nil {
			o.RaftPeers = make(map[string]string)
		}
		o.RaftPeers[name] = addr
	}
}

func WithRaftPeers(m map[string]string) Option {
	return func(o *Options) {
		o.RaftPeers = m
	}
}

func WithCodecType(t string) Option {
	return func(o *Options) {
		o.CodecType = t
	}
}

func WithElectionTimeout(t time.Duration) Option {
	return func(o *Options) {
		o.ElectionTimeout = t
	}
}

func WithHeartBeatTimeout(t time.Duration) Option {
	return func(o *Options) {
		o.HeartBeatTimeout = t
	}
}

func WithApplyInterval(t time.Duration) Option {
	return func(o *Options) {
		o.ApplyInterval = t
	}
}

func WithRPCTimeout(t time.Duration) Option {
	return func(o *Options) {
		o.RPCTimeout = t
	}
}
