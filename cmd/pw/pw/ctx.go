// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package pw

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/config"
	"github.com/getpatchwork/patchwork/pkg/db"
)

type (
	configKey  struct{}
	dbKey      struct{}
	versionKey struct{}
)

func WithConfig(ctx context.Context, cfg *config.Config) context.Context {
	return context.WithValue(ctx, configKey{}, cfg)
}

func WithDB(ctx context.Context, d *bun.DB) context.Context {
	return context.WithValue(ctx, dbKey{}, d)
}

func WithVersion(ctx context.Context, version string) context.Context {
	return context.WithValue(ctx, versionKey{}, version)
}

func GetConfig(ctx context.Context) *config.Config {
	return ctx.Value(configKey{}).(*config.Config)
}

func GetDB(ctx context.Context) *bun.DB {
	return ctx.Value(dbKey{}).(*bun.DB)
}

func GetVersion(ctx context.Context) string {
	return ctx.Value(versionKey{}).(string)
}

func NewQueries(ctx context.Context) *db.Queries {
	return db.New(ctx, GetDB(ctx))
}

func BeginTx(ctx context.Context) (*db.Queries, error) {
	return db.Begin(ctx, GetDB(ctx))
}
