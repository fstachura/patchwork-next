// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/danielgtaylor/huma/v2"
	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/db"
	"github.com/getpatchwork/patchwork/pkg/log"
)

func registerEventRoutes(api huma.API, h *handler, prefix string, mw huma.Middlewares) {
	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: prefix + "/events",
		OperationID: fmt.Sprintf("list-events-v%s", prefix[5:]),
		Middlewares: mw,
	}, h.ListEvents)
}

type ListEventsInput struct {
	PageParams
	Project  string `query:"project" doc:"Project ID or linkname"`
	Category string `query:"category" doc:"Event category"`
	Patch    int    `query:"patch" doc:"Patch ID"`
	Cover    int    `query:"cover" doc:"Cover ID"`
	Series   int    `query:"series" doc:"Series ID"`
	Actor    int    `query:"actor" doc:"Actor user ID"`
	Since    string `query:"since" doc:"Earliest date"`
	Before   string `query:"before" doc:"Latest date"`
	Order    string `query:"order" doc:"Ordering (date, -date)"`
}

type ListEventsOutput struct {
	Link string          `header:"Link" doc:"Pagination links"`
	Body []EventResponse `json:"body"`
}

func (h *handler) ListEvents(
	ctx context.Context, input *ListEventsInput,
) (*ListEventsOutput, error) {
	base := h.apiBase(ctx)

	q := db.GetQueries(ctx)
	sq := q.Select((*db.Event)(nil))
	sq = applyEventFilters(sq, input)

	total, err := sq.Count(ctx)
	if err != nil {
		log.Errorf("count events: %v", err)
		return nil, huma.Error500InternalServerError("Internal error.")
	}

	offset, perPage := input.offsetPage(h.cfg)

	order := "event.date DESC"
	if input.Order == "date" {
		order = "event.date ASC"
	}

	var events []db.Event
	if err := sq.Model(&events).
		Relation("Project").
		Relation("Actor").
		Relation("Patch", db.WithColumns("id", "msgid", "date", "name")).
		Relation("Series", db.WithColumns("id", "name", "date", "version")).
		Relation("Cover", db.WithColumns("id", "msgid", "date", "name")).
		Relation("PatchComment", db.WithColumns("id", "msgid", "date")).
		Relation("CoverComment", db.WithColumns("id", "msgid", "date")).
		Relation("PreviousState", db.WithColumns("id", "slug")).
		Relation("CurrentState", db.WithColumns("id", "slug")).
		OrderExpr(order).Offset(offset).Limit(perPage).Scan(ctx); err != nil {
		log.Errorf("list events: %v", err)
		return nil, huma.Error500InternalServerError("Internal error.")
	}

	resp := &ListEventsOutput{
		Link: buildLinkHeader(input.Page, perPage, total),
		Body: make([]EventResponse, len(events)),
	}
	for i := range events {
		resp.Body[i] = eventToResponse(&events[i], ctx, q, base)
	}
	return resp, nil
}

func applyEventFilters(q *bun.SelectQuery, input *ListEventsInput) *bun.SelectQuery {
	if input.Project != "" {
		if id, err := strconv.Atoi(input.Project); err == nil {
			q = q.Where("event.project_id = ?", id)
		} else {
			q = q.Where("event.project_id IN (SELECT id FROM project WHERE linkname = ?)", input.Project)
		}
	}
	if input.Category != "" {
		q = q.Where("event.category = ?", input.Category)
	}
	if input.Patch != 0 {
		q = q.Where("event.patch_id = ?", input.Patch)
	}
	if input.Cover != 0 {
		q = q.Where("event.cover_id = ?", input.Cover)
	}
	if input.Series != 0 {
		q = q.Where("event.series_id = ?", input.Series)
	}
	if input.Actor != 0 {
		q = q.Where("event.actor_id = ?", input.Actor)
	}
	if input.Since != "" {
		q = q.Where("event.date >= ?", input.Since)
	}
	if input.Before != "" {
		q = q.Where("event.date < ?", input.Before)
	}
	return q
}

func buildEventPayload(ctx context.Context, q *db.Queries, e *db.Event) map[string]any {
	m := map[string]any{}

	if e.Patch != nil {
		p := e.Patch
		m["patch"] = map[string]any{
			"id": p.ID, "msgid": p.Msgid,
			"date": p.Date, "name": p.Name,
		}
	}
	if e.Series != nil {
		s := e.Series
		m["series"] = map[string]any{
			"id": s.ID, "name": s.Name,
			"date": s.Date, "version": s.Version,
		}
	}
	if e.Cover != nil {
		c := e.Cover
		m["cover"] = map[string]any{
			"id": c.ID, "msgid": c.Msgid,
			"date": c.Date, "name": c.Name,
		}
	}
	if e.PatchComment != nil {
		c := e.PatchComment
		m["comment"] = map[string]any{
			"id": c.ID, "msgid": c.Msgid, "date": c.Date,
		}
	}
	if e.CoverComment != nil {
		c := e.CoverComment
		m["comment"] = map[string]any{
			"id": c.ID, "msgid": c.Msgid, "date": c.Date,
		}
	}
	if e.PreviousState != nil {
		s := e.PreviousState
		m["previous_state"] = s.Slug
	}
	if e.CurrentState != nil {
		s := e.CurrentState
		m["current_state"] = s.Slug
	}

	if len(m) == 0 {
		return nil
	}
	return m
}

func eventToResponse(e *db.Event, ctx context.Context, q *db.Queries, base string) EventResponse {
	r := EventResponse{
		ID:       e.ID,
		Category: e.Category,
		Date:     e.Date,
		Payload:  buildEventPayload(ctx, q, e),
	}
	if e.Project != nil {
		r.Project = projectToEmbedded(e.Project, base)
	}
	if e.Actor != nil {
		a := userToEmbedded(e.Actor, base)
		r.Actor = &a
	}
	return r
}
