// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package admin

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/cmd/pw/pw"
	"github.com/getpatchwork/patchwork/pkg/db"
	"github.com/getpatchwork/patchwork/pkg/log"
	"github.com/getpatchwork/patchwork/pkg/mail"
)

type LabelCmd struct {
	List    LabelListCmd   `cmd:"" help:"List labels."`
	Create  LabelCreateCmd `cmd:"" help:"Create a label."`
	Update  LabelUpdateCmd `cmd:"" help:"Update a label."`
	Delete  LabelDeleteCmd `cmd:"" help:"Delete a label."`
	Relabel RelabelCmd     `cmd:"" help:"Relabel existing patches from subject prefixes."`
}

type LabelListCmd struct{}

func (c *LabelListCmd) Run(ctx *pw.Context) error {
	var labels []db.Label
	err := ctx.DB.NewSelect().Model(&labels).
		OrderExpr("id ASC").
		Scan(ctx)
	if err != nil {
		return err
	}

	q := db.New(ctx, ctx.DB)
	projectNames := make(map[int]string)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "ID\tNAME\tPROJECT\tCOLOR\tDESCRIPTION\n")
	for _, l := range labels {
		proj := "(global)"
		if l.ProjectID != nil {
			name, ok := projectNames[*l.ProjectID]
			if !ok {
				p, err := q.GetProjectByID(*l.ProjectID)
				if err == nil {
					name = p.Linkname
				} else {
					name = fmt.Sprintf("?%d", *l.ProjectID)
				}
				projectNames[*l.ProjectID] = name
			}
			proj = name
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t#%06x\t%s\n",
			l.ID, l.Name, proj, l.Color, l.Description)
	}
	return w.Flush()
}

type LabelCreateCmd struct {
	Name        string `required:"" short:"n" help:"Label name."`
	Project     string `short:"p" help:"Project linkname (omit for global)."`
	Description string `short:"d" help:"Label description."`
	Color       int    `short:"c" default:"0" help:"Color as integer (e.g. 0xff0000 for red)."`
}

func (c *LabelCreateCmd) Run(ctx *pw.Context) error {
	label := db.Label{
		Name:        c.Name,
		Description: c.Description,
		Color:       c.Color,
	}
	if c.Project != "" {
		q := db.New(ctx, ctx.DB)
		project, err := q.GetProjectByLinkname(c.Project)
		if err != nil {
			return fmt.Errorf("project %q not found", c.Project)
		}
		label.ProjectID = &project.ID
	}

	err := db.New(ctx, ctx.DB).Insert(&label)
	if err != nil {
		return err
	}

	fmt.Printf("Created label %q (id=%d)\n", label.Name, label.ID)
	return nil
}

type LabelUpdateCmd struct {
	Name        string `arg:"" help:"Label name to update."`
	Project     string `short:"p" help:"Move label to this project linkname (use 'global' to make global)."`
	Color       *int   `short:"c" help:"New color as integer."`
	Description string `short:"d" help:"New description."`
	Rename      string `short:"r" help:"Rename the label."`
}

func (c *LabelUpdateCmd) Run(ctx *pw.Context) error {
	var label db.Label
	err := ctx.DB.NewSelect().Model(&label).
		Where("name = ?", c.Name).
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("label %q not found", c.Name)
	}

	uq := ctx.DB.NewUpdate().Model(&label).Where("id = ?", label.ID)
	changed := false

	if c.Project != "" {
		if strings.EqualFold(c.Project, "global") {
			uq = uq.Set("project_id = NULL")
		} else {
			q := db.New(ctx, ctx.DB)
			project, err := q.GetProjectByLinkname(c.Project)
			if err != nil {
				return fmt.Errorf("project %q not found", c.Project)
			}
			uq = uq.Set("project_id = ?", project.ID)
		}
		changed = true
	}
	if c.Color != nil {
		uq = uq.Set("color = ?", *c.Color)
		changed = true
	}
	if c.Description != "" {
		uq = uq.Set("description = ?", c.Description)
		changed = true
	}
	if c.Rename != "" {
		uq = uq.Set("name = ?", c.Rename)
		changed = true
	}

	if !changed {
		fmt.Println("Nothing to update.")
		return nil
	}

	if _, err := uq.Exec(ctx); err != nil {
		return err
	}

	fmt.Printf("Updated label %q\n", c.Name)
	return nil
}

type LabelDeleteCmd struct {
	Force bool   `short:"f" help:"Skip confirmation."`
	Name  string `arg:"" help:"Label name to delete."`
}

func (c *LabelDeleteCmd) Run(ctx *pw.Context) error {
	var label db.Label
	err := ctx.DB.NewSelect().Model(&label).
		Where("name = ?", c.Name).
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("label %q not found", c.Name)
	}

	if !c.Force {
		fmt.Printf("Delete label %q (id=%d)? [y/N] ", label.Name, label.ID)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	_, err = ctx.DB.NewDelete().Model((*db.Label)(nil)).
		Where("id = ?", label.ID).
		Exec(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("Deleted label %q\n", c.Name)
	return nil
}

type RelabelCmd struct {
	Projects []string `arg:"" optional:"" help:"Project listIDs to relabel (all if omitted)."`
}

func (c *RelabelCmd) Run(ctx *pw.Context) error {
	var labels []db.Label
	if err := ctx.DB.NewSelect().Model(&labels).Scan(ctx); err != nil {
		return err
	}
	if len(labels) == 0 {
		fmt.Println("No labels defined.")
		return nil
	}

	type labelKey struct {
		name      string
		projectID int
	}
	labelMap := make(map[labelKey]*db.Label)
	for i := range labels {
		pid := 0
		if labels[i].ProjectID != nil {
			pid = *labels[i].ProjectID
		}
		labelMap[labelKey{strings.ToLower(labels[i].Name), pid}] = &labels[i]
	}

	findLabel := func(name string, projectID int) *db.Label {
		if l, ok := labelMap[labelKey{strings.ToLower(name), projectID}]; ok {
			return l
		}
		return labelMap[labelKey{strings.ToLower(name), 0}]
	}

	insertLabel, err := ctx.DB.PrepareContext(ctx,
		"INSERT INTO patch_label (patch_id, label_id) VALUES (?, ?) ON CONFLICT (patch_id, label_id) DO NOTHING")
	if err != nil {
		return err
	}
	defer insertLabel.Close()

	updateName, err := ctx.DB.PrepareContext(ctx,
		"UPDATE patch SET name = ? WHERE id = ?")
	if err != nil {
		return err
	}
	defer updateName.Close()

	const batchSize = 1000
	lastID := 0
	total := 0
	updated := 0

	for {
		sq := ctx.DB.NewSelect().Model((*db.Patch)(nil)).
			Column("id", "name", "headers", "project_id").
			Where("id > ?", lastID).
			OrderExpr("id ASC").
			Limit(batchSize)
		if len(c.Projects) > 0 {
			sq = sq.Where(
				"project_id IN (SELECT id FROM project WHERE listid IN ?)",
				bun.Tuple(c.Projects),
			)
		}

		var patches []db.Patch
		if err := sq.Scan(ctx, &patches); err != nil {
			return err
		}
		if len(patches) == 0 {
			break
		}

		for _, patch := range patches {
			lastID = patch.ID
			total++

			subject := mail.ParseSubjectFromHeaders(patch.Headers)
			if subject == "" {
				continue
			}

			_, prefixes := mail.CleanSubject(subject, nil)
			if len(prefixes) == 0 {
				continue
			}

			var matched []db.Label
			var remaining []string
			for _, pfx := range prefixes {
				if l := findLabel(pfx, patch.ProjectID); l != nil {
					matched = append(matched, *l)
				} else {
					remaining = append(remaining, pfx)
				}
			}
			if len(matched) == 0 {
				continue
			}

			newName := mail.RebuildSubject(
				mail.StripPrefixes(patch.Name), remaining,
			)
			if newName != patch.Name {
				if _, err := updateName.ExecContext(ctx, newName, patch.ID); err != nil {
					log.Warnf("update patch %d name: %v", patch.ID, err)
				}
			}

			for _, l := range matched {
				if _, err := insertLabel.ExecContext(ctx, patch.ID, l.ID); err != nil {
					log.Warnf("set label for patch %d: %v", patch.ID, err)
				}
			}
			updated++
		}

		fmt.Printf("\rprocessed %d patches, %d updated", total, updated)
	}

	fmt.Printf("\rprocessed %d patches, %d updated\n", total, updated)
	return nil
}
