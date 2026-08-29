// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"time"

	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/cmd/pw/pw"
	"github.com/getpatchwork/patchwork/pkg/db"
)

type GenerateCmd struct {
	Series  int `required:"" help:"Number of series to generate."`
	Authors int `required:"" help:"Number of authors to generate."`
	Labels  int `required:"" help:"Number of labels."`
}

const BATCH_SIZE = 1000

func (c *GenerateCmd) Run(ctx *pw.Context) error {
	name := fmt.Sprintf("project_%d", rand.Int())

	project := db.Project{
		Name:                 name,
		Linkname:             name,
		Listid:               name,
		Listemail:            fmt.Sprintf("%s@example.com", name),
		WebURL:               "",
		ScmURL:               "",
		WebScmURL:            "",
		ListArchiveURL:       "",
		SubjectMatch:         "",
		CommitURLFormat:      "",
		ListArchiveURLFormat: "",
	}
	err := db.New(ctx, ctx.DB).Insert(&project)
	if err != nil {
		return err
	}

	fmt.Printf("Created project %q (id=%d)\n", project.Linkname, project.ID)

	var states []db.State
	err = ctx.DB.NewSelect().Model(&states).Scan(ctx)
	if err != nil {
		return err
	}

	var tags []db.Tag
	err = ctx.DB.NewSelect().Model(&tags).Scan(ctx)
	if err != nil {
		return err
	}

	var authors []db.Person
	for _ = range c.Authors {
		name := fmt.Sprintf("person_%d", rand.Int())

		person := db.Person{
			Email: name + "@example.com",
			Name:  &name,
		}
		err := db.New(ctx, ctx.DB).Insert(&person)
		if err != nil {
			return err
		}

		authors = append(authors, person)
	}

	var labels []db.Label
	for l := range c.Labels {
		name := fmt.Sprintf("label_%d", l)

		label := db.Label{
			ProjectID: &project.ID,
			Name:      name,
			Color:     randColor(),
		}
		err := db.New(ctx, ctx.DB).Insert(&label)
		if err != nil {
			return err
		}

		labels = append(labels, label)
	}

	series_batches := c.Series / BATCH_SIZE
	for sb := range series_batches {
		batch_size := BATCH_SIZE

		err := ctx.DB.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			var batch_covers []db.Cover
			for b := range batch_size {
				batch_covers = append(
					batch_covers,
					randCover(project, authors[rand.Int()%len(authors)], sb*BATCH_SIZE+b),
				)
			}

			_, err = tx.NewInsert().Model(&batch_covers).Exec(ctx)
			if err != nil {
				return err
			}

			var batch_series []db.Series
			for _, cover := range batch_covers {
				batch_series = append(batch_series, randSeries(project, cover))
			}
			_, err = tx.NewInsert().Model(&batch_series).Exec(ctx)
			if err != nil {
				return err
			}

			var batch_patches []db.Patch
			for s, series := range batch_series {
				for p := range series.Total {
					name := fmt.Sprintf("series %d patch %d", sb*BATCH_SIZE+s, p)
					batch_patches = append(
						batch_patches,
						randPatch(project, series, states[rand.Int()%len(states)], name, p),
					)
				}
			}

			_, err = tx.NewInsert().Model(&batch_patches).Exec(ctx)
			if err != nil {
				return err
			}

			var batch_labels []db.PatchLabel
			var batch_tags []db.PatchTag
			var batch_checks []db.Check
			for _, patch := range batch_patches {
				for l := range len(labels) {
					if rand.Int()%(l+2) != l {
						continue
					}

					batch_labels = append(batch_labels, db.PatchLabel{
						PatchID: patch.ID,
						LabelID: labels[l].ID,
					})
				}

				for t := range len(tags) {
					if rand.Int()%(t+2) != t {
						continue
					}

					batch_tags = append(batch_tags, db.PatchTag{
						PatchID: patch.ID,
						TagID:   tags[t].ID,
						Count:   rand.Int() % 10,
					})
				}

				for _ = range rand.Int() % 10 {
					batch_checks = append(batch_checks, randCheck(patch))
				}
			}

			_, err = tx.NewInsert().Model(&batch_labels).Exec(ctx)
			if err != nil {
				return err
			}

			_, err = tx.NewInsert().Model(&batch_tags).Exec(ctx)
			if err != nil {
				return err
			}

			_, err = tx.NewInsert().Model(&batch_checks).Exec(ctx)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			return err
		}

		fmt.Printf("\rgenerated series %d/%d", sb*BATCH_SIZE, c.Series)
	}
	fmt.Printf("\n")

	return nil
}

func randString(len uint32) string {
	res := make([]byte, len)
	for i := range len {
		t := rand.Uint32() % 3
		if t == 0 {
			// 0-9
			res[i] = byte(48 + (rand.Uint32() % 10))
		} else if t == 1 {
			// A-Z
			res[i] = byte(65 + (rand.Uint32() % 25))
		} else {
			// a-z
			res[i] = byte(97 + (rand.Uint32() % 25))
		}
	}

	return string(res)
}

func randColor() int {
	r := (rand.Int() % 200) + 40
	g := (rand.Int() % 200) + 40
	b := (rand.Int() % 200) + 40

	return (r << 16) | (g << 8) | b
}

func randCover(project db.Project, submitter db.Person, num int) db.Cover {
	name := fmt.Sprintf("cover %d", num)
	content := randString(rand.Uint32() % 5000)
	headers := randString(rand.Uint32() % 2000)
	msgid := fmt.Sprintf("<%s@example.com>", randString(15))

	return db.Cover{
		Msgid:       msgid,
		Date:        time.Now(),
		Headers:     headers,
		SubmitterID: submitter.ID,
		Content:     &content,
		ProjectID:   project.ID,
		Name:        name,
	}
}

func randSeries(project db.Project, cover db.Cover) db.Series {
	patches := rand.Int() % 20

	return db.Series{
		ProjectID:     &project.ID,
		CoverLetterID: &cover.ID,
		Name:          &cover.Name,
		Date:          time.Now(),
		SubmitterID:   cover.SubmitterID,
		Version:       0,
		Total:         patches,

		ReceivedTotal: patches,
		ReceivedAll:   true,
	}
}

func randPatch(project db.Project, series db.Series, state db.State, name string, num int) db.Patch {
	msgid := fmt.Sprintf("<%s@example.com>", randString(15))
	headers := randString(1000)
	content := randString(5000)
	diff := ""
	hash := randString(32)

	return db.Patch{
		Msgid:       msgid,
		Date:        time.Now(),
		Headers:     headers,
		SubmitterID: series.SubmitterID,
		Content:     &content,
		ProjectID:   project.ID,
		Name:        name,
		Diff:        &diff,
		StateID:     &state.ID,
		Archived:    (rand.Int() % 10) >= 9,
		Hash:        &hash,
		SeriesID:    &series.ID,
		Number:      &num,
	}
}

func randCheck(patch db.Patch) db.Check {
	return db.Check{
		PatchID:     patch.ID,
		Date:        time.Now(),
		State:       db.CheckState(rand.Int() % 4),
		TargetURL:   "",
		Context:     fmt.Sprintf("check_%d", rand.Int()%5),
		Description: "",
	}
}
