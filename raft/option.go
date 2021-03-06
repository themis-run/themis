package raft

import (
	"time"

	"go.themis.run/themis/codec"
)

type Options struct {
	NativeName        string
	Address           string        `yaml:"address"`
	CodecType         string        `yaml:"codec_type"`
	ApplyMsgLength    int           `yaml:"apply_msg_length"`
	SnapshotPath      string        `yaml:"snapshot_path"`
	MaxLogEntryLength int           `yaml:"max_log_entry_length"`
	ElectionTimeout   time.Duration `yaml:"election_timeout"`
	HeartBeatTimeout  time.Duration `yaml:"heartbeat_timeout"`
	ApplyInterval     time.Duration `yaml:"apply_interval"`
	RPCTimeout        time.Duration `yaml:"rpc_timeout"`
	RaftPeers         map[string]string
	InfoCh            chan *RaftInfo
}

func DefaultOptions() *Options {
	return &Options{
		CodecType:         codec.Gob,
		SnapshotPath:      "./snapshot",
		MaxLogEntryLength: 20,
		ApplyMsgLength:    10,
		ElectionTimeout:   900 * time.Millisecond,
		HeartBeatTimeout:  450 * time.Millisecond,
		ApplyInterval:     300 * time.Millisecond,
		RPCTimeout:        300 * time.Millisecond,
	}
}

type Option func(*Options)

func WithNativeName(name string) Option {
	return func(o *Options) {
		o.NativeName = name
	}
}

func WithAddress(address string) Option {
	return func(o *Options) {
		o.Address = address
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

func WithSnapshotPath(path string) Option {
	return func(o *Options) {
		o.SnapshotPath = path
	}
}

func WithMaxlogEntryLength(length int) Option {
	return func(o *Options) {
		o.MaxLogEntryLength = length
	}
}
