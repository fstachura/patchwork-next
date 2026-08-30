// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package http

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/getpatchwork/patchwork/cmd/pw/pw"
	"github.com/getpatchwork/patchwork/pkg/api"
	"github.com/getpatchwork/patchwork/pkg/db/migrations"
	"github.com/getpatchwork/patchwork/pkg/events"
	"github.com/getpatchwork/patchwork/pkg/log"
	"github.com/getpatchwork/patchwork/pkg/mbox"
	"github.com/getpatchwork/patchwork/pkg/web"
)

type CLI struct{}

func (c *CLI) Run(ctx context.Context) error {
	cfg := pw.GetConfig(ctx)
	database := pw.GetDB(ctx)
	version := pw.GetVersion(ctx)

	if cfg.Database.AutoSync {
		if err := migrations.RunMigrations(ctx, database); err != nil {
			return err
		}
	} else if err := migrations.CheckSchemaVersion(ctx, database); err != nil {
		return err
	}

	bus := events.Start(ctx, database)
	defer bus.Shutdown()

	mbox.Version = version

	router := web.NewRouter(cfg, database, bus, version)
	router.Mount("/", api.NewRouter(cfg, database, bus))

	srv := &http.Server{
		Addr:     cfg.Http.Listen,
		Handler:  router,
		ErrorLog: log.ErrLogger(),
	}
	sock, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	log.Noticef("patchwork %s listening on http://%s", version, sock.Addr())

	unregister := context.AfterFunc(ctx, func() {
		log.Noticef("%s, shutting down", context.Cause(ctx))
		timeout, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if e := srv.Shutdown(timeout); e != nil {
			log.Errorf("shutdown: %v", e)
		}
	})
	defer unregister()

	if err = srv.Serve(sock); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}

	return nil
}
