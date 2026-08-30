// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"context"
	"fmt"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/alecthomas/kong"

	"github.com/getpatchwork/patchwork/cmd/pw/admin"
	pwcfg "github.com/getpatchwork/patchwork/cmd/pw/config"
	pwdb "github.com/getpatchwork/patchwork/cmd/pw/db"
	"github.com/getpatchwork/patchwork/cmd/pw/http"
	"github.com/getpatchwork/patchwork/cmd/pw/ingress"
	"github.com/getpatchwork/patchwork/cmd/pw/pw"
	"github.com/getpatchwork/patchwork/pkg/config"
	"github.com/getpatchwork/patchwork/pkg/db"
	"github.com/getpatchwork/patchwork/pkg/log"
)

type CLI struct {
	config.Config

	ShowVersion VersionFlag `name:"version" help:"Print patchwork version and exit."`

	Cfg     pwcfg.CLI   `cmd:"" name:"config" help:"Configuration utilities."`
	Admin   admin.CLI   `cmd:"" help:"Administration CLI."`
	DB      pwdb.CLI    `cmd:"" help:"Database management."`
	Ingress ingress.CLI `cmd:"" help:"Ingress SMTP/LMTP daemon."`
	Http    http.CLI    `cmd:"" help:"HTTP server daemon."`
}

// set at build time
var (
	Version string
	Date    string
)

type VersionFlag bool

func (v VersionFlag) BeforeReset(app *kong.Kong, vars kong.Vars) error {
	fmt.Printf("patchwork %s (%s %s %s %s)\n",
		Version, runtime.Version(), runtime.GOARCH, runtime.GOOS, Date)
	app.Exit(0)
	return nil
}

func main() {
	var cli CLI

	config.RegisterHints("events", admin.EventCategories())

	k := config.Parse(&cli, "Patchwork runtime commands.")

	if cli.Syslog {
		log.InitSyslog("pw-" + k.Command())
		k.Stderr = log.ErrLogger().Writer()
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx = pw.WithConfig(ctx, &cli.Config)
	ctx = pw.WithVersion(ctx, Version)

	if !strings.HasPrefix(k.Command(), "config") {
		database, err := db.Open(&cli.Config)
		k.FatalIfErrorf(err, "database")
		defer database.Close()
		ctx = pw.WithDB(ctx, database)
	}

	k.BindTo(ctx, (*context.Context)(nil))
	k.FatalIfErrorf(k.Run())
}
