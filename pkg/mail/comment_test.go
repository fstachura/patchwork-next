// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package mail

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommentCorrelation(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	patchMsgID := "<patch-1@test>"
	coverMsgID := "<cover-1@test>"

	parseEmail(t, ctx, database, sampleDiff,
		withMsgID(patchMsgID),
		withSubject("[PATCH 1/1] test patch"),
		withListID("test.example.com"))

	parseEmail(t, ctx, database, "cover letter body",
		withMsgID(coverMsgID),
		withSubject("[PATCH 0/1] test cover"),
		withListID("test.example.com"))

	t.Run("direct patch reply", func(t *testing.T) {
		err := parseEmail(t, ctx, database, "looks good\nReviewed-by: Me <me@test>",
			withSubject("Re: [PATCH 1/1] test patch"),
			withInReplyTo(patchMsgID),
			withListID("test.example.com"))
		require.NoError(t, err)
		assert.Equal(t, 1, countPatchComments(t, database), "expected 1 patch comment")
	})

	t.Run("no reply ref", func(t *testing.T) {
		err := parseEmail(t, ctx, database, "orphan comment",
			withSubject("Re: something unrelated"),
			withListID("test.example.com"))
		require.NoError(t, err)
		assert.Equal(t, 1, countPatchComments(t, database), "expected still 1 patch comment")
	})
}

func TestCommentCorrelationFull(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	patchMsgID := "<corr-patch@test>"
	coverMsgID := "<corr-cover@test>"
	commentOnPatchMsgID := "<corr-comment-patch@test>"
	commentOnCoverMsgID := "<corr-comment-cover@test>"

	parseEmail(t, ctx, database, sampleDiff,
		withMsgID(patchMsgID),
		withSubject("[PATCH 1/1] test"),
		withListID("test.example.com"))
	parseEmail(t, ctx, database, "cover body",
		withMsgID(coverMsgID),
		withSubject("[PATCH 0/1] cover"),
		withListID("test.example.com"))

	parseEmail(t, ctx, database, "patch comment",
		withMsgID(commentOnPatchMsgID),
		withSubject("Re: [PATCH 1/1] test"),
		withInReplyTo(patchMsgID),
		withListID("test.example.com"))

	parseEmail(t, ctx, database, "cover comment",
		withMsgID(commentOnCoverMsgID),
		withSubject("Re: [PATCH 0/1] cover"),
		withInReplyTo(coverMsgID),
		withListID("test.example.com"))

	t.Run("patch comment direct reply", func(t *testing.T) {
		assert.Equal(t, 1, countPatchComments(t, database))
	})

	t.Run("cover comment direct reply", func(t *testing.T) {
		var count int
		database.NewSelect().TableExpr("cover_comment").
			ColumnExpr("count(*)").
			Scan(context.Background(), &count)
		assert.Equal(t, 1, count)
	})

	t.Run("indirect patch reply via comment", func(t *testing.T) {
		parseEmail(t, ctx, database, "reply to comment",
			withSubject("Re: Re: [PATCH 1/1] test"),
			withInReplyTo(commentOnPatchMsgID),
			withListID("test.example.com"))
		assert.Equal(t, 2, countPatchComments(t, database))
	})

	t.Run("no matching parent", func(t *testing.T) {
		before := countPatchComments(t, database)
		parseEmail(t, ctx, database, "orphan comment",
			withSubject("Re: unrelated"),
			withInReplyTo("<nonexistent@test>"),
			withListID("test.example.com"))
		assert.Equal(t, before, countPatchComments(t, database), "orphan comment should not create a patch comment")
	})
}

func TestCommentOnCorrectParent(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	patch1 := "<parent-1@test>"
	patch2 := "<parent-2@test>"

	parseEmail(t, ctx, database, sampleDiff,
		withMsgID(patch1), withSubject("[PATCH 1/2] first"),
		withListID("test.example.com"))
	parseEmail(t, ctx, database, sampleDiff,
		withMsgID(patch2), withSubject("[PATCH 2/2] second"),
		withInReplyTo(patch1),
		withListID("test.example.com"))

	parseEmail(t, ctx, database, "looks good",
		withSubject("Re: [PATCH 2/2] second"),
		withInReplyTo(patch2),
		withListID("test.example.com"))

	var parentMsgID string
	database.NewRaw(`
		SELECT p.msgid FROM patch_comment c
		JOIN patch p ON p.id = c.patch_id
		LIMIT 1
	`).Scan(context.Background(), &parentMsgID)
	assert.Equal(t, patch2, parentMsgID)
}

func TestCommentActionRequired(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	patchMsgID := "<action-patch@test>"

	parseEmail(t, ctx, database, sampleDiff,
		withMsgID(patchMsgID),
		withSubject("[PATCH 1/1] test"),
		withListID("test.example.com"))

	commentA := "<comment-a@test>"
	parseEmail(t, ctx, database, "test comment\n",
		withMsgID(commentA),
		withSubject("Re: [PATCH 1/1] test"),
		withInReplyTo(patchMsgID),
		withListID("test.example.com"))

	commentB := "<comment-b@test>"
	parseEmail(t, ctx, database, "another test comment\n",
		withMsgID(commentB),
		withSubject("Re: [PATCH 1/1] test"),
		withInReplyTo(patchMsgID),
		withListID("test.example.com"),
		withHeader("X-Patchwork-Action-Required", ""))

	require.Equal(t, 2, countPatchComments(t, database))

	var addressedA, addressedB *bool
	database.NewSelect().TableExpr("patch_comment").
		Column("addressed").Where("msgid = ?", commentA).
		Scan(context.Background(), &addressedA)
	database.NewSelect().TableExpr("patch_comment").
		Column("addressed").Where("msgid = ?", commentB).
		Scan(context.Background(), &addressedB)

	assert.Nil(t, addressedA, "comment A addressed should be NULL")
	require.NotNil(t, addressedB)
	assert.False(t, *addressedB)
}

func TestCoverCommentActionRequired(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	coverMsgID := "<action-cover@test>"
	parseEmail(t, ctx, database, "cover body",
		withMsgID(coverMsgID),
		withSubject("[PATCH 0/1] test cover"),
		withListID("test.example.com"))

	commentMsgID := "<action-cover-comment@test>"
	parseEmail(t, ctx, database, "comment with action",
		withMsgID(commentMsgID),
		withSubject("Re: [PATCH 0/1] test cover"),
		withInReplyTo(coverMsgID),
		withListID("test.example.com"),
		withHeader("X-Patchwork-Action-Required", ""))

	var addressed *bool
	database.NewSelect().TableExpr("cover_comment").
		Column("addressed").Where("msgid = ?", commentMsgID).
		Scan(context.Background(), &addressed)
	require.NotNil(t, addressed)
	assert.False(t, *addressed)
}

func TestCoverCommentActionRequiredFull(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	coverMsgID := "<action-full-cover@test>"
	parseEmail(t, ctx, database, "cover body",
		withMsgID(coverMsgID),
		withSubject("[PATCH 0/1] test"),
		withListID("test.example.com"))

	commentA := "<action-full-a@test>"
	parseEmail(t, ctx, database, "normal comment",
		withMsgID(commentA),
		withSubject("Re: [PATCH 0/1] test"),
		withInReplyTo(coverMsgID),
		withListID("test.example.com"))

	commentB := "<action-full-b@test>"
	parseEmail(t, ctx, database, "action comment",
		withMsgID(commentB),
		withSubject("Re: [PATCH 0/1] test"),
		withInReplyTo(coverMsgID),
		withListID("test.example.com"),
		withHeader("X-Patchwork-Action-Required", ""))

	var addrA, addrB *bool
	database.NewSelect().TableExpr("cover_comment").
		Column("addressed").Where("msgid = ?", commentA).
		Scan(context.Background(), &addrA)
	database.NewSelect().TableExpr("cover_comment").
		Column("addressed").Where("msgid = ?", commentB).
		Scan(context.Background(), &addrB)

	assert.Nil(t, addrA, "comment A addressed should be NULL")
	require.NotNil(t, addrB)
	assert.False(t, *addrB)
}

func TestCoverCommentIndirectReply(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	coverMsgID := "<indirect-cover@test>"
	parseEmail(t, ctx, database, "cover body",
		withMsgID(coverMsgID),
		withSubject("[PATCH 0/1] test"),
		withListID("test.example.com"))

	comment1MsgID := "<indirect-comment1@test>"
	parseEmail(t, ctx, database, "first comment",
		withMsgID(comment1MsgID),
		withSubject("Re: [PATCH 0/1] test"),
		withInReplyTo(coverMsgID),
		withListID("test.example.com"))

	parseEmail(t, ctx, database, "reply to comment",
		withSubject("Re: Re: [PATCH 0/1] test"),
		withInReplyTo(comment1MsgID),
		withListID("test.example.com"))

	var count int
	database.NewSelect().TableExpr("cover_comment").
		ColumnExpr("count(*)").Scan(context.Background(), &count)
	assert.GreaterOrEqual(t, count, 2, "indirect cover comment reply")
}

func TestMultipleProjectComment(t *testing.T) {
	database, ctx, _, _ := testDB(t, "project-a.example.com")
	testProject(t, database, "project-b", "Project B", "project-b.example.com", "")

	patchMsgID := "<multi-proj-patch@test>"

	parseEmail(t, ctx, database, sampleDiff,
		withMsgID(patchMsgID),
		withSubject("[PATCH] test"),
		withListID("project-a.example.com"))

	parseEmail(t, ctx, database, "comment on project A",
		withSubject("Re: [PATCH] test"),
		withInReplyTo(patchMsgID),
		withListID("project-a.example.com"))

	assert.Equal(t, 1, countPatchComments(t, database))
}
