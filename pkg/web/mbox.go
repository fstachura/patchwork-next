// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package web

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/db"
	"github.com/getpatchwork/patchwork/pkg/mbox"
)

func (h *webHandler) PatchMboxPage(w http.ResponseWriter, r *http.Request) {
	linkname := urlParam(r, "linkname")
	rawMsgid := urlParam(r, "msgid")
	ctx := r.Context()
	q := db.GetQueries(ctx)
	msgid := "<" + rawMsgid + ">"

	var patch db.Patch
	err := q.DB.NewSelect().
		Model(&patch).
		Join("JOIN project AS pr ON pr.id = patch.project_id").
		Where("pr.linkname = ?", linkname).
		Where("patch.msgid = ?", msgid).
		Scan(ctx)
	if err != nil {
		notFoundPage(w)
		return
	}

	var project db.Project
	err = q.DB.NewSelect().Model(&project).Where("id = ?", patch.ProjectID).Scan(q.Ctx)
	if err != nil {
		serverErrorPage(w, "get project", err)
		return
	}

	seriesParam := r.URL.Query().Get("series")
	if seriesParam != "" {
		h.seriesPatchMbox(w, r, patch, project, seriesParam)
		return
	}

	h.servePatchMbox(w, patch, project)
}

func (h *webHandler) servePatchMbox(w http.ResponseWriter, patch db.Patch, project db.Project) {
	ctx := context.Background()
	sub := mbox.BuildPatchSubmission(ctx, h.db, &patch, project.Listemail)
	body := mbox.Format(sub)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%s.patch", mbox.SanitizeFilename(patch.Name)))
	_, _ = w.Write(body)
}

func (h *webHandler) CoverMboxPage(w http.ResponseWriter, r *http.Request) {
	linkname := urlParam(r, "linkname")
	rawMsgid := urlParam(r, "msgid")
	ctx := r.Context()
	q := db.GetQueries(ctx)
	msgid := "<" + rawMsgid + ">"

	var cover db.Cover
	err := q.DB.NewSelect().
		Model(&cover).
		Join("JOIN project AS pr ON pr.id = cover.project_id").
		Where("pr.linkname = ?", linkname).
		Where("cover.msgid = ?", msgid).
		Scan(ctx)
	if err != nil {
		notFoundPage(w)
		return
	}

	var project db.Project
	err = q.DB.NewSelect().Model(&project).Where("id = ?", cover.ProjectID).Scan(q.Ctx)
	if err != nil {
		serverErrorPage(w, "get project", err)
		return
	}

	h.serveCoverMbox(w, cover, project)
}

func (h *webHandler) serveCoverMbox(w http.ResponseWriter, cover db.Cover, project db.Project) {
	ctx := context.Background()
	sub := mbox.BuildCoverSubmission(ctx, h.db, &cover, project.Listemail)
	body := mbox.Format(sub)

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%s.mbox", mbox.SanitizeFilename(cover.Name)))
	_, _ = w.Write(body)
}

func (h *webHandler) SeriesMbox(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := db.GetQueries(ctx)
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		notFoundPage(w)
		return
	}

	var series db.Series
	err = q.DB.NewSelect().Model(&series).Where("id = ?", id).Scan(q.Ctx)
	if err != nil {
		notFoundPage(w)
		return
	}

	var project db.Project
	if series.ProjectID != nil {
		if err = q.DB.NewSelect().Model(&project).Where("id = ?", *series.ProjectID).Scan(q.Ctx); err != nil {
			serverErrorPage(w, "get project", err)
			return
		}
	}

	var patches []db.Patch
	err = q.DB.NewSelect().Model(&patches).
		Where("series_id = ?", series.ID).
		OrderBy("number", bun.OrderAsc).
		Scan(ctx)
	if err != nil {
		serverErrorPage(w, "list series patches", err)
		return
	}

	var parts [][]byte
	for i := range patches {
		sub := mbox.BuildPatchSubmission(ctx, q.DB, &patches[i], project.Listemail)
		parts = append(parts, mbox.Format(sub))
	}

	name := "series"
	if series.Name != nil {
		name = *series.Name
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%s.patch", mbox.SanitizeFilename(name)))
	_, _ = w.Write(bytes.Join(parts, []byte("\n")))
}

func (h *webHandler) seriesPatchMbox(w http.ResponseWriter, r *http.Request, patch db.Patch, project db.Project, seriesParam string) {
	ctx := r.Context()
	q := db.GetQueries(ctx)

	if patch.SeriesID == nil {
		notFoundPage(w)
		return
	}

	if seriesParam != "*" {
		sid, err := strconv.ParseInt(seriesParam, 10, 32)
		if err != nil || int(sid) != *patch.SeriesID {
			notFoundPage(w)
			return
		}
	}

	var deps []db.Patch
	if patch.Number != nil {
		q.DB.NewSelect().Model(&deps).
			Where("series_id = ?", *patch.SeriesID).
			Where("number < ?", *patch.Number).
			OrderBy("number", bun.OrderAsc).
			Scan(ctx)
	}

	var parts [][]byte
	for i := range deps {
		sub := mbox.BuildPatchSubmission(ctx, q.DB, &deps[i], project.Listemail)
		parts = append(parts, mbox.Format(sub))
	}

	sub := mbox.BuildPatchSubmission(ctx, q.DB, &patch, project.Listemail)
	parts = append(parts, mbox.Format(sub))

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%s.patch", mbox.SanitizeFilename(patch.Name)))
	_, _ = w.Write(bytes.Join(parts, []byte("\n")))
}

func (h *webHandler) BundleMbox(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := db.GetQueries(ctx)
	username := urlParam(r, "username")
	bundlename := urlParam(r, "bundlename")

	var bundle db.Bundle
	err := q.DB.NewSelect().
		Model(&bundle).
		Join("JOIN auth_user AS u ON u.id = bundle.owner_id").
		Where("u.username = ?", username).
		Where("bundle.name = ?", bundlename).
		Scan(ctx)
	if err != nil {
		notFoundPage(w)
		return
	}

	if !bundle.Public {
		notFoundPage(w)
		return
	}

	var project db.Project
	if err = q.DB.NewSelect().Model(&project).Where("id = ?", bundle.ProjectID).Scan(q.Ctx); err != nil {
		serverErrorPage(w, "get project", err)
		return
	}

	var patches []db.Patch
	err = q.DB.NewSelect().
		Model(&patches).
		Join("JOIN bundle_patch AS bp ON bp.patch_id = patch.id").
		Where("bp.bundle_id = ?", bundle.ID).
		OrderBy("bp.order", bun.OrderAsc).
		Scan(ctx)
	if err != nil {
		serverErrorPage(w, "list bundle patches", err)
		return
	}

	var parts [][]byte
	for i := range patches {
		sub := mbox.BuildPatchSubmission(ctx, q.DB, &patches[i], project.Listemail)
		parts = append(parts, mbox.Format(sub))
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=bundle-%d-%s.mbox",
			bundle.ID, mbox.SanitizeFilename(bundle.Name)))
	_, _ = w.Write(bytes.Join(parts, []byte("\n")))
}

func (h *webHandler) CommentRedirect(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := db.GetQueries(ctx)
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		notFoundPage(w)
		return
	}

	// try patch comment first
	var pc struct {
		PatchID int
	}
	err = q.DB.NewSelect().Model((*db.PatchComment)(nil)).Column("patch_id").
		Where("id = ?", id).
		Scan(ctx, &pc)
	if err == nil {
		var patch struct {
			Msgid     string
			ProjectID int
		}
		q.DB.NewSelect().Model((*db.Patch)(nil)).Column("msgid", "project_id").
			Where("id = ?", pc.PatchID).
			Scan(ctx, &patch)
		var linkname string
		q.DB.NewSelect().Model((*db.Project)(nil)).Column("linkname").
			Where("id = ?", patch.ProjectID).
			Scan(ctx, &linkname)
		http.Redirect(w, r,
			patchURL(linkname, patch.Msgid)+fmt.Sprintf("#comment-%d", id),
			http.StatusMovedPermanently)
		return
	}

	// try cover comment
	var cc struct {
		CoverID int
	}
	err = q.DB.NewSelect().Model((*db.CoverComment)(nil)).Column("cover_id").
		Where("id = ?", id).
		Scan(ctx, &cc)
	if err == nil {
		var cover struct {
			Msgid     string
			ProjectID int
		}
		q.DB.NewSelect().Model((*db.Cover)(nil)).Column("msgid", "project_id").
			Where("id = ?", cc.CoverID).
			Scan(ctx, &cover)
		var linkname string
		q.DB.NewSelect().Model((*db.Project)(nil)).Column("linkname").
			Where("id = ?", cover.ProjectID).
			Scan(ctx, &linkname)
		http.Redirect(w, r,
			coverURL(linkname, cover.Msgid)+fmt.Sprintf("#comment-%d", id),
			http.StatusMovedPermanently)
		return
	}

	notFoundPage(w)
}
