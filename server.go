package main

import (
	"dns-switch/internal/config"
	"dns-switch/internal/dns"
	"dns-switch/internal/server"
)

// runServer starts the HTTP management panel.
func runServer() error {
	srv := server.New(config.FileStore{}, dns.New(config.FileStore{}))
	return srv.ListenAndServe("127.0.0.1:9753")
}
