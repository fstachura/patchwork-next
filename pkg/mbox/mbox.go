// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package mbox

import (
	"bufio"
	"bytes"
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-mbox"
	"github.com/emersion/go-message/mail"
	"github.com/emersion/go-message/textproto"
	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/db"
	"github.com/getpatchwork/patchwork/pkg/log"
)

var Version string

var (
	postscriptRe   = regexp.MustCompile(`(?m)^-{2,3} ?$`)
	mboxResponseRe = regexp.MustCompile(`(?mi)^(Tested|Reviewed|Acked|Signed-off|Nacked|Reported)-by:.*$`)
	signatureRe    = regexp.MustCompile(`(?m)^-- $`)
	safeFilenameRe = regexp.MustCompile(`[^a-z0-9_.-]+`)
)

type Submission struct {
	ID              int
	Date            time.Time
	Content         string
	Diff            string
	Headers         string
	Submitter       mail.Address
	DelegateEmail   string
	ListEmail       string
	CommentContents []string
}

func Format(sub Submission) []byte {
	h, err := textproto.ReadHeader(bufio.NewReader(
		strings.NewReader(sub.Headers + "\n")))
	if err != nil {
		log.Errorf("textproto.ReadHeader: %v", err)
		h = textproto.Header{}
	}

	if fromVal := h.Get("From"); fromVal != "" {
		fromAddr, err := mail.ParseAddress(fromVal)
		if err == nil && fromAddr.Address == sub.ListEmail {
			h.Set("X-Patchwork-Original-From", fromVal)
			h.Set("From", sub.Submitter.String())
		}
	}
	if !h.Has("Date") {
		h.Set("Date", sub.Date.UTC().Format(time.RFC1123Z))
	}
	h.Set("X-Patchwork-Submitter", sub.Submitter.String())
	h.Set("X-Patchwork-Id", strconv.Itoa(sub.ID))
	if sub.Diff != "" && sub.DelegateEmail != "" {
		h.Set("X-Patchwork-Delegate", sub.DelegateEmail)
	}
	h.Set("Content-Type", "text/plain; charset=utf-8")
	h.Set("Content-Transfer-Encoding", "8bit")

	var buf bytes.Buffer

	mw := mbox.NewWriter(&buf)

	w, err := mw.CreateMessage("patchwork", sub.Date)
	if err != nil {
		log.Errorf("mbox.CreateMessage: %v", err)
		return nil
	}

	if err := textproto.WriteHeader(w, h); err != nil {
		log.Errorf("textproto.WriteHeader: %v", err)
		return nil
	}

	body := ""
	if sub.Content != "" {
		body = strings.TrimSpace(sub.Content) + "\n"
	}

	postscript := ""
	if loc := postscriptRe.FindStringIndex(body); loc != nil {
		postscript = body[loc[1]:]
		body = strings.TrimSpace(body[:loc[0]]) + "\n"
		postscript = strings.TrimRight(postscript, " \t\n")
	}

	for _, content := range sub.CommentContents {
		for _, m := range mboxResponseRe.FindAllString(content, -1) {
			body += m + "\n"
		}
	}

	if postscript != "" {
		body += "---" + postscript + "\n"
	} else if sub.Diff != "" {
		body += "---\n"
	}

	if sub.Diff != "" {
		body += "\n" + sub.Diff
	}

	if Version != "" && !signatureRe.MatchString(body) {
		body += "-- \npatchwork " + Version + "\n"
	}

	_, _ = w.Write([]byte(body))
	mw.Close()

	return buf.Bytes()
}

func SanitizeFilename(name string) string {
	s := strings.ToLower(name)
	s = safeFilenameRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "patch"
	}
	return s
}

func BuildPatchSubmission(
	ctx context.Context, idb bun.IDB, patch *db.Patch, listEmail string,
) Submission {
	sub := Submission{
		ID:        patch.ID,
		Date:      patch.Date,
		Headers:   patch.Headers,
		ListEmail: listEmail,
	}
	if patch.Content != nil {
		sub.Content = *patch.Content
	}
	if patch.Diff != nil {
		sub.Diff = *patch.Diff
	}

	var submitter db.Person
	if idb.NewSelect().Model(&submitter).
		Where("id = ?", patch.SubmitterID).
		Scan(ctx) == nil {
		sub.Submitter.Address = submitter.Email
		if submitter.Name != nil {
			sub.Submitter.Name = *submitter.Name
		}
	}

	if patch.DelegateID != nil {
		var delegate db.User
		if idb.NewSelect().Model(&delegate).
			Where("id = ?", *patch.DelegateID).
			Scan(ctx) == nil {
			sub.DelegateEmail = delegate.Email
		}
	}

	var contents []string
	if err := idb.NewSelect().
		Model((*db.PatchComment)(nil)).Column("content").
		Where("patch_id = ?", patch.ID).OrderExpr("date ASC").
		Scan(ctx, &contents); err != nil {
		log.Errorf("load patch comments: %v", err)
	}
	sub.CommentContents = contents

	return sub
}

func BuildCoverSubmission(
	ctx context.Context, idb bun.IDB, cover *db.Cover, listEmail string,
) Submission {
	sub := Submission{
		ID:        cover.ID,
		Date:      cover.Date,
		Headers:   cover.Headers,
		ListEmail: listEmail,
	}
	if cover.Content != nil {
		sub.Content = *cover.Content
	}

	var submitter db.Person
	if idb.NewSelect().Model(&submitter).
		Where("id = ?", cover.SubmitterID).
		Scan(ctx) == nil {
		sub.Submitter.Address = submitter.Email
		if submitter.Name != nil {
			sub.Submitter.Name = *submitter.Name
		}
	}

	var contents []string
	if err := idb.NewSelect().
		Model((*db.CoverComment)(nil)).Column("content").
		Where("cover_id = ?", cover.ID).OrderExpr("date ASC").
		Scan(ctx, &contents); err != nil {
		log.Errorf("load cover comments: %v", err)
	}
	sub.CommentContents = contents

	return sub
}
