// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package web

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/getpatchwork/patchwork/pkg/db"
)

func (h *webHandler) CoverDetailPage(w http.ResponseWriter, r *http.Request) {
	linkname := urlParam(r, "linkname")
	rawMsgid := urlParam(r, "msgid")
	ctx := r.Context()
	q := db.GetQueries(ctx)
	msgid := "<" + rawMsgid + ">"

	project, err := q.GetProjectByLinkname(linkname)
	if err != nil {
		notFoundPage(w)
		return
	}

	var cover db.Cover
	err = q.DB.NewSelect().Model(&cover).
		Relation("Submitter").
		Where("project_id = ?", project.ID).
		Where("msgid = ?", msgid).
		Scan(q.Ctx)
	if err != nil {
		notFoundPage(w)
		return
	}

	var series *db.Series
	if cover.ID > 0 {
		var s db.Series
		if q.DB.NewSelect().Model(&s).Where("cover_letter_id = ?", cover.ID).Scan(q.Ctx) == nil {
			series = &s
		}
	}

	var seriesPatches []seriesPatchRef
	metadata := make(map[string]string)

	if series != nil {
		sPatches, err := q.ListSeriesPatches(series.ID)
		if err != nil {
			serverErrorPage(w, "list series patches", err)
			return
		}
		for _, sp := range sPatches {
			seriesPatches = append(seriesPatches, seriesPatchRef{
				Name: sp.Name,
				URL:  patchURL(project.Linkname, sp.Msgid),
			})
		}

		metadata, err = q.GetSeriesMetadata(series.ID)
		if err != nil {
			serverErrorPage(w, "list series metadata", err)
			return
		}
	}

	comments, err := q.ListCoverComments(cover.ID)
	if err != nil {
		serverErrorPage(w, "list comments", err)
		return
	}

	data := coverDetailData{
		PC:       h.projectPageCtx(r, project),
		Project:  *project,
		Cover:    cover,
		Comments: comments,
		Series:   series,
		Patches:  seriesPatches,
		Metadata: metadata,
	}
	coverDetailPage(data).Render(ctx, w)
}

func (h *webHandler) CoverRedirect(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := db.GetQueries(ctx)
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		notFoundPage(w)
		return
	}

	var cover struct {
		Msgid     string
		ProjectID int
	}
	err = q.DB.NewSelect().Model((*db.Cover)(nil)).
		Column("msgid", "project_id").
		Where("id = ?", id).
		Scan(q.Ctx, &cover)
	if err != nil {
		notFoundPage(w)
		return
	}

	project, err := q.GetProjectByID(cover.ProjectID)
	if err != nil {
		notFoundPage(w)
		return
	}

	http.Redirect(w, r, coverURL(project.Linkname, cover.Msgid), http.StatusMovedPermanently)
}

func (h *webHandler) CoverMboxByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := db.GetQueries(ctx)
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		notFoundPage(w)
		return
	}

	cover, err := q.GetCoverByID(int(id))
	if err != nil {
		notFoundPage(w)
		return
	}

	project, err := q.GetProjectByID(cover.ProjectID)
	if err != nil {
		notFoundPage(w)
		return
	}

	h.serveCoverMbox(w, *cover, *project)
}
