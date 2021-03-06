package config

import (
	"errors"
	"io/ioutil"
	"os"

	"go.themis.run/themis/logging"
	"go.themis.run/themis/raft"

	"gopkg.in/yaml.v3"
)

var ErrorConfigFileNotExist = errors.New("config file not exist")

type Config struct {
	Name          string             `yaml:"name"`
	Address       string             `yaml:"address"`
	Path          string             `yaml:"path"`
	Size          uint               `yaml:"size"`
	Raft          *raft.Options      `yaml:"raft"`
	Log           *logging.Options   `yaml:"logging"`
	PeerAddress   map[string]Address `yaml:"peers"`
	ServerAddress map[string]string
}

type Address struct {
	RaftAddress   string `yaml:"cluster_address"`
	ListenAddress string `yaml:"listen_address"`
}

func Create(path string) Config {
	config := &Config{
		Path: "./log",
		Size: 16,
		Raft: raft.DefaultOptions(),
		Log:  logging.DefaultOptions,
	}

	config.loadYaml(path)

	config.Raft.NativeName = config.Name
	config.initRaftPeer()

	return *config
}

func (c *Config) initRaftPeer() {
	peers := make(map[string]string)
	servers := make(map[string]string)

	for k, v := range c.PeerAddress {
		peers[k] = v.RaftAddress
		servers[k] = v.ListenAddress
	}

	servers[c.Name] = c.Address
	c.Raft.RaftPeers = peers
	c.ServerAddress = servers
}

func pathExist(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		return os.IsExist(err)
	}
	return true
}

func readConfigBytes(path string) ([]byte, error) {
	if path == "" {
		path = "./themis.yml"
	}

	if !pathExist(path) {
		return nil, ErrorConfigFileNotExist
	}
	return ioutil.ReadFile(path)
}

func (c *Config) loadYaml(path string) {
	bytes, err := readConfigBytes(path)
	if err != nil {
		logging.Error(err)
		return
	}

	if err := yaml.Unmarshal(bytes, c); err != nil {
		logging.Error(err)
	}
}
