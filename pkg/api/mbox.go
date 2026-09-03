// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/db"
	"github.com/getpatchwork/patchwork/pkg/mbox"
)

func (h *handler) patchMbox(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := db.GetQueries(ctx)
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	var patch db.Patch
	if err := q.Select(&patch).
		Where("id = ?", id).Scan(ctx); err != nil {
		http.NotFound(w, r)
		return
	}

	var project db.Project
	if err := q.Select(&project).
		Where("id = ?", patch.ProjectID).Scan(ctx); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	sub := mbox.BuildPatchSubmission(ctx, q.DB, &patch, project.Listemail)
	body := mbox.Format(sub)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%s.patch",
			mbox.SanitizeFilename(patch.Name)))
	_, _ = w.Write(body)
}

func (h *handler) coverMbox(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := db.GetQueries(ctx)
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	var cover db.Cover
	if err := q.Select(&cover).
		Where("id = ?", id).Scan(ctx); err != nil {
		http.NotFound(w, r)
		return
	}

	var project db.Project
	if err := q.Select(&project).
		Where("id = ?", cover.ProjectID).Scan(ctx); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	sub := mbox.BuildCoverSubmission(ctx, q.DB, &cover, project.Listemail)
	body := mbox.Format(sub)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%s.mbox",
			mbox.SanitizeFilename(cover.Name)))
	_, _ = w.Write(body)
}

func (h *handler) seriesMbox(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := db.GetQueries(ctx)
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	var series db.Series
	if err := q.Select(&series).
		Where("id = ?", id).Scan(ctx); err != nil {
		http.NotFound(w, r)
		return
	}

	var project db.Project
	if series.ProjectID != nil {
		if err := q.Select(&project).
			Where("id = ?", *series.ProjectID).Scan(ctx); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}

	var patches []db.Patch
	if err := q.Select(&patches).
		Where("series_id = ?", series.ID).
		OrderBy("number", bun.OrderAsc).
		Scan(ctx); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	name := "series"
	if series.Name != nil {
		name = *series.Name
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%s.patch",
			mbox.SanitizeFilename(name)))
	for i := range patches {
		sub := mbox.BuildPatchSubmission(ctx, q.DB, &patches[i], project.Listemail)
		_, _ = w.Write(mbox.Format(sub))
	}
}

func (h *handler) bundleMbox(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := db.GetQueries(ctx)
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	var bundle db.Bundle
	if err := q.Select(&bundle).
		Where("id = ?", id).Scan(ctx); err != nil {
		http.NotFound(w, r)
		return
	}

	user := getAuthUser(ctx)
	if !bundle.Public && (user == nil || user.ID != bundle.OwnerID) {
		http.NotFound(w, r)
		return
	}

	var project db.Project
	if err := q.Select(&project).
		Where("id = ?", bundle.ProjectID).Scan(ctx); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	var patches []db.Patch
	if err := q.Select(&patches).
		Join("JOIN bundle_patch AS bp ON bp.patch_id = patch.id").
		Where("bp.bundle_id = ?", bundle.ID).
		OrderBy("bp.order", bun.OrderAsc).
		Scan(ctx); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=bundle-%d-%s.mbox",
			bundle.ID, mbox.SanitizeFilename(bundle.Name)))
	for i := range patches {
		sub := mbox.BuildPatchSubmission(ctx, q.DB, &patches[i], project.Listemail)
		_, _ = w.Write(mbox.Format(sub))
	}
}
