// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/getpatchwork/patchwork/cmd/pw/pw"
	"github.com/getpatchwork/patchwork/pkg/db/migrations"
)

type StatusCmd struct{}

func (c *StatusCmd) Run(ctx context.Context) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "SCHEMA_MIGRATION\tAPPLIED\n")
	for _, m := range migrations.ListMigrations(ctx, pw.GetDB(ctx)) {
		if m.AppliedAt.IsZero() {
			fmt.Fprintf(w, "%s\t%s\n", m.Name, "")
		} else {
			fmt.Fprintf(w, "%s\t%s\n", m.Name, m.AppliedAt)
		}
	}
	return w.Flush()
}
