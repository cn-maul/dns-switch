package main

import (
	"embed"

	"dns-switch/internal/config"
	"dns-switch/internal/dns"
	"dns-switch/internal/service"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed frontend/*.html frontend/*.css frontend/*.js
var assets embed.FS

func runApp() {
	cfgStore := config.FileStore{}
	dnsMgr := dns.New(cfgStore)
	svc := service.NewDNSService(cfgStore, dnsMgr)

	app := application.New(application.Options{
		Name: "DNS-Switch",
		Services: []application.Service{
			application.NewService(svc),
		},
		Assets: application.AssetOptions{
			FS: assets,
		},
	})

	app.NewWebviewWindowWithOptions(application.WebviewWindowOptions{
		Title:  "DNS-Switch",
		Width:  700,
		Height: 600,
	})

	app.Run()
}
