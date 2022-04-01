package main

import (
	"flag"

	"go.themis.run/themis/config"
	"go.themis.run/themis/server"
)

func main() {
	var path string
	flag.StringVar(&path, "path", "./themis.yml", "config path")
	flag.Parse()

	server.New(config.Create(path)).Run()
}
