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

func TestBaseSeriesSinglePatch(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/base-single-patch.mbox", "test.example.com")
	assert.Equal(t, 1, result.patches)
	assert.Equal(t, 1, result.series)
	assertAllPatchesHaveSeries(t, database)
}

func TestBaseSeriesCoverLetter(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/base-cover-letter.mbox", "test.example.com")
	assert.Equal(t, 2, result.patches)
	assert.Equal(t, 1, result.covers)
	assert.Equal(t, 1, result.series)
	assertAllPatchesInOneSeries(t, database)
	assertCoverLinkedToSeries(t, database)
}

func TestBaseSeriesNoCoverLetter(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/base-no-cover-letter.mbox", "test.example.com")
	assert.Equal(t, 2, result.patches)
	assert.Equal(t, 0, result.covers)
	assert.Equal(t, 1, result.series)
	assertSerialized(t, database, []int{2})
}

func TestBaseSeriesDeepThreaded(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/base-deep-threaded.mbox", "test.example.com")
	assert.Equal(t, 2, result.patches)
	assert.Equal(t, 1, result.covers)
	assert.Equal(t, 1, result.series)
}

func TestBaseSeriesOutOfOrder(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/base-out-of-order.mbox", "test.example.com")
	assert.Equal(t, 2, result.patches)
	assert.Equal(t, 1, result.covers)
	assert.Equal(t, 1, result.series)
}

func TestBaseSeriesIncomplete(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/base-incomplete.mbox", "test.example.com")
	assert.Equal(t, 1, result.patches)
	assert.Equal(t, 1, result.covers)
	assert.Equal(t, 1, result.series)
}

func TestBaseSeriesDifferentVersions(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/base-different-versions.mbox", "test.example.com")
	assert.Equal(t, 1, result.covers)
	assert.Equal(t, 4, result.patches)
}

func TestBaseSeriesNoReferences(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/base-no-references.mbox", "test.example.com")
	assert.Equal(t, 2, result.patches)
}

func TestBaseSeriesNoReferencesNoCover(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/base-no-references-no-cover.mbox", "test.example.com")
	assert.Equal(t, 1, result.covers)
	assert.Equal(t, 2, result.patches)
}

func TestBaseSeriesExtraPatches(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/base-extra-patches.mbox", "test.example.com")
	assert.Equal(t, 1, result.covers)
	assert.Equal(t, 3, result.patches)
}

func TestBugsMultipleReferences(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/bugs-multiple-references.mbox", "test.example.com")
	assert.Equal(t, 1, result.covers)
	assert.Equal(t, 4, result.patches)
}

func TestBugsMultipleContentTypes(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/bugs-multiple-content-types.mbox", "test.example.com")
	assert.Equal(t, 1, result.patches)
	assert.Equal(t, 1, result.patchComments)
}

func TestBugsNocover(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/bugs-nocover.mbox", "test.example.com")
	assert.Equal(t, 4, result.patches)
}

func TestBugsNocoverNoversion(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/bugs-nocover-noversion.mbox", "test.example.com")
	assert.Equal(t, 4, result.patches)
}

func TestBugsSpamming(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/bugs-spamming.mbox", "test.example.com")
	assert.Equal(t, 3, result.patches)
}

func TestBugsUnnumbered(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/bugs-unnumbered.mbox", "test.example.com")
	assert.Equal(t, 1, result.covers)
	assert.Equal(t, 2, result.patches)
}

func TestBugsMixedVersions(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/bugs-mixed-versions.mbox", "test.example.com")
	assert.Equal(t, 2, result.patches)
}

func TestRevisionBasic(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/revision-basic.mbox", "test.example.com")
	assert.Equal(t, 4, result.patches)
	assert.Equal(t, 2, result.covers)
	assert.Equal(t, 2, result.series)
	assertSerialized(t, database, []int{2, 2})
}

func TestRevisionThreadedToSinglePatch(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/revision-threaded-to-single-patch.mbox", "test.example.com")
	assert.Equal(t, 2, result.patches)
}

func TestRevisionThreadedToCover(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/revision-threaded-to-cover.mbox", "test.example.com")
	assert.Equal(t, 2, result.covers)
	assert.Equal(t, 4, result.patches)
	assertSerialized(t, database, []int{2, 2})
}

func TestRevisionThreadedToPatch(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/revision-threaded-to-patch.mbox", "test.example.com")
	assert.Equal(t, 2, result.covers)
	assert.Equal(t, 4, result.patches)
	assertSerialized(t, database, []int{2, 2})
}

func TestRevisionOutOfOrder(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/revision-out-of-order.mbox", "test.example.com")
	assert.Equal(t, 2, result.covers)
	assert.Equal(t, 4, result.patches)
	assertSerialized(t, database, []int{2, 2})
}

func TestRevisionNoCoverLetter(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/revision-no-cover-letter.mbox", "test.example.com")
	assert.Equal(t, 1, result.covers)
	assert.Equal(t, 4, result.patches)
	assertSerialized(t, database, []int{2, 2})
}

func TestRevisionUnlabeled(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/revision-unlabeled.mbox", "test.example.com")
	assert.Equal(t, 2, result.covers)
	assert.Equal(t, 4, result.patches)
	assertSerialized(t, database, []int{2, 2})
}

func TestRevisionUnlabeledNoreferences(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/revision-unlabeled-noreferences.mbox", "test.example.com")
	assert.Equal(t, 4, result.patches)
	assertSerialized(t, database, []int{2, 2})
}

func TestRevisedSeriesReplyNocover(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/bugs-nocover.mbox", "test.example.com")
	assert.Equal(t, 4, result.patches)
	assert.GreaterOrEqual(t, result.series, 2)
}

func TestRevisedSeriesReplyNocoverNoversion(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/bugs-nocover-noversion.mbox", "test.example.com")
	assert.Equal(t, 4, result.patches)
}

func TestMercurialCoverLetter(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/mercurial-cover-letter.mbox", "test.example.com")
	assert.Equal(t, 2, result.patches)
	assert.Equal(t, 1, result.covers)
	assert.Equal(t, 1, result.series)
}

func TestMercurialNoCoverLetter(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/mercurial-no-cover-letter.mbox", "test.example.com")
	assert.Equal(t, 2, result.patches)
	assert.Equal(t, 1, result.series)
}

func TestSeriesCorrelation(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	t.Run("new series", func(t *testing.T) {
		parseEmail(t, ctx, database, sampleDiff,
			withSubject("[PATCH 1/2] first patch"),
			withMsgID("<series-1-patch-1@test>"),
			withListID("test.example.com"))
		assert.Equal(t, 1, countPatches(t, database))

		var seriesCount int
		database.NewSelect().TableExpr("series").
			ColumnExpr("count(*)").
			Scan(context.Background(), &seriesCount)
		assert.Equal(t, 1, seriesCount)
	})

	t.Run("reply joins series", func(t *testing.T) {
		parseEmail(t, ctx, database, sampleDiff,
			withSubject("[PATCH 2/2] second patch"),
			withMsgID("<series-1-patch-2@test>"),
			withInReplyTo("<series-1-patch-1@test>"),
			withListID("test.example.com"))
		require.Equal(t, 2, countPatches(t, database))

		var seriesCount int
		database.NewSelect().TableExpr("series").
			ColumnExpr("count(*)").
			Scan(context.Background(), &seriesCount)
		require.Equal(t, 1, seriesCount)

		assertAllPatchesInOneSeries(t, database)
	})
}

func TestSeriesName(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	t.Run("cover letter sets name", func(t *testing.T) {
		result := parseMbox(t, ctx, database, "series/base-cover-letter.mbox", "test.example.com")
		require.Equal(t, 1, result.series)

		var name string
		database.NewSelect().TableExpr("series").
			Column("name").Limit(1).
			Scan(context.Background(), &name)
		assert.NotEmpty(t, name, "series name should be set from cover letter")
	})
}

func TestSeriesNameCoverLetter(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/base-cover-letter.mbox", "test.example.com")
	require.Equal(t, 1, result.series)

	var name string
	database.NewSelect().TableExpr("series").
		Column("name").Limit(1).
		Scan(context.Background(), &name)
	assert.NotEmpty(t, name, "series name should be set from cover letter")
	assert.Equal(t, "A sample series", name)
}

func TestSeriesNameNoCoverLetter(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/base-no-cover-letter.mbox", "test.example.com")
	require.Equal(t, 1, result.series)

	var seriesName, firstPatchName string
	database.NewSelect().TableExpr("series").
		Column("name").Limit(1).
		Scan(context.Background(), &seriesName)
	database.NewSelect().TableExpr("patch").
		Column("name").Where("number = 1").Limit(1).
		Scan(context.Background(), &firstPatchName)
	assert.NotEmpty(t, seriesName, "series name should be set from first patch")
	assert.Equal(t, firstPatchName, seriesName)
}

func TestSeriesNameOutOfOrder(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/base-out-of-order.mbox", "test.example.com")
	require.Equal(t, 1, result.series)

	var name string
	database.NewSelect().TableExpr("series").
		Column("name").Limit(1).
		Scan(context.Background(), &name)
	assert.NotEmpty(t, name, "series name should be set")
}

func TestSeriesNameCustom(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/base-cover-letter.mbox", "test.example.com")
	require.Equal(t, 1, result.series)

	database.NewRaw("UPDATE series SET name = 'Custom Name'").
		Exec(context.Background())

	var name string
	database.NewSelect().TableExpr("series").
		Column("name").Limit(1).
		Scan(context.Background(), &name)
	assert.Equal(t, "Custom Name", name)
}

func TestSeriesTotalComplete(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/base-cover-letter.mbox", "test.example.com")
	assert.Equal(t, 2, result.patches)

	var total, patchCount int
	database.NewSelect().TableExpr("series").
		Column("total").Limit(1).
		Scan(context.Background(), &total)
	database.NewSelect().TableExpr("patch").
		ColumnExpr("count(*)").Where("series_id IS NOT NULL").
		Scan(context.Background(), &patchCount)
	assert.GreaterOrEqual(t, patchCount, total, "series not complete")
}

func TestSeriesReceivedAll(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	t.Run("complete", func(t *testing.T) {
		result := parseMbox(t, ctx, database, "series/base-cover-letter.mbox", "test.example.com")
		require.Equal(t, 1, result.series)

		var total int
		var patchCount int
		database.NewSelect().TableExpr("series").
			Column("total").Limit(1).
			Scan(context.Background(), &total)
		database.NewSelect().TableExpr("patch").
			ColumnExpr("count(*)").Where("series_id IS NOT NULL").
			Scan(context.Background(), &patchCount)
		assert.GreaterOrEqual(t, patchCount, total, "series not complete")
	})

	t.Run("incomplete", func(t *testing.T) {
		db2, ctx2, _, _ := testDB(t, "test.example.com")
		result := parseMbox(t, ctx2, db2, "series/base-incomplete.mbox", "test.example.com")
		require.Equal(t, 1, result.series)

		var total int
		var patchCount int
		db2.NewSelect().TableExpr("series").
			Column("total").Limit(1).
			Scan(context.Background(), &total)
		db2.NewSelect().TableExpr("patch").
			ColumnExpr("count(*)").Where("series_id IS NOT NULL").
			Scan(context.Background(), &patchCount)
		assert.Less(t, patchCount, total, "series should be incomplete")
	})
}

func TestNestedSeries(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	parseEmail(t, ctx, database, "cover body",
		withMsgID("<nested-v1-cover@test>"),
		withSubject("[PATCH 0/2] first series"),
		withListID("test.example.com"))
	parseEmail(t, ctx, database, sampleDiff,
		withMsgID("<nested-v1-p1@test>"),
		withSubject("[PATCH 1/2] first patch"),
		withInReplyTo("<nested-v1-cover@test>"),
		withListID("test.example.com"))
	parseEmail(t, ctx, database, sampleDiff,
		withMsgID("<nested-v1-p2@test>"),
		withSubject("[PATCH 2/2] second patch"),
		withInReplyTo("<nested-v1-cover@test>"),
		withListID("test.example.com"))

	parseEmail(t, ctx, database, "v2 cover body",
		withMsgID("<nested-v2-cover@test>"),
		withSubject("[PATCH v2 0/2] first series"),
		withInReplyTo("<nested-v1-cover@test>"),
		withListID("test.example.com"))
	parseEmail(t, ctx, database, sampleDiff,
		withMsgID("<nested-v2-p1@test>"),
		withSubject("[PATCH v2 1/2] first patch"),
		withInReplyTo("<nested-v2-cover@test>"),
		withListID("test.example.com"))

	var seriesCount int
	database.NewSelect().TableExpr("series").
		ColumnExpr("count(*)").
		Scan(context.Background(), &seriesCount)
	assert.GreaterOrEqual(t, seriesCount, 2)
}

func TestPreviousSeriesLinkage(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/revision-basic.mbox", "test.example.com")
	require.Equal(t, 2, result.series)

	var series []struct {
		ID               int
		Version          int
		PreviousSeriesID *int
	}
	database.NewSelect().TableExpr("series").
		Column("id", "version", "previous_series_id").
		OrderExpr("version").
		Scan(context.Background(), &series)

	require.Len(t, series, 2)

	v1 := series[0]
	v2 := series[1]

	assert.Equal(t, 1, v1.Version)
	assert.Equal(t, 2, v2.Version)

	require.NotNil(t, v2.PreviousSeriesID, "v2 series should have previous_series_id set")
	assert.Equal(t, v1.ID, *v2.PreviousSeriesID)

	assert.Nil(t, v1.PreviousSeriesID, "v1 series should not have previous_series_id")
}

func TestPreviousSeriesThreaded(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/revision-threaded-to-cover.mbox", "test.example.com")
	require.Equal(t, 2, result.series)

	var count int
	database.NewSelect().TableExpr("series").
		ColumnExpr("count(*)").
		Where("previous_series_id IS NOT NULL").
		Scan(context.Background(), &count)
	assert.Equal(t, 1, count)
}

func TestPreviousSeriesNameSimilarity(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	parseEmail(t, ctx, database, sampleDiff,
		withSubject("[PATCH] fix: improve error handling"),
		withMsgID("<v1-patch@test>"),
		withListID("test.example.com"))

	parseEmail(t, ctx, database, sampleDiff,
		withSubject("[PATCH v2] fix: improve error handling"),
		withMsgID("<v2-patch@test>"),
		withListID("test.example.com"))

	var count int
	database.NewSelect().TableExpr("series").
		ColumnExpr("count(*)").
		Where("previous_series_id IS NOT NULL").
		Scan(context.Background(), &count)
	assert.Equal(t, 1, count)
}

func TestCoverNamePriority(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	parseEmail(t, ctx, database, sampleDiff,
		withSubject("[PATCH 1/2] the patch name"),
		withMsgID("<covername-1@test>"),
		withListID("test.example.com"))

	patchName := "[1/2] the patch name"
	name := getSeriesName(t, database)
	assert.Equal(t, patchName, name)

	parseEmail(t, ctx, database, "cover body\n",
		withSubject("[PATCH 0/2] the cover name"),
		withMsgID("<covername-0@test>"),
		withInReplyTo("<covername-1@test>"),
		withListID("test.example.com"))

	name = getSeriesName(t, database)
	assert.Equal(t, "the cover name", name)
}
