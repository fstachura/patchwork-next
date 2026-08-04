// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package mbox

import (
	"bufio"
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	gombox "github.com/emersion/go-mbox"
	"github.com/emersion/go-message/mail"
	"github.com/emersion/go-message/textproto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func baseSubmission() Submission {
	return Submission{
		ID:   42,
		Date: time.Date(1981, 6, 23, 16, 45, 0, 0, time.UTC),
		Headers: "From: Alice <alice@example.com>\n" +
			"Subject: [PATCH] fix something\n" +
			"Message-Id: <123@example.com>\n",
		Submitter: mail.Address{
			Name:    "Alice",
			Address: "alice@example.com",
		},
		Content: "This is the commit message.\n\nSigned-off-by: Alice <alice@example.com>\n",
		Diff:    "diff --git a/file.c b/file.c\n--- a/file.c\n+++ b/file.c\n@@ -1 +1 @@\n-old\n+new\n",
	}
}

func parseMbox(t *testing.T, data []byte) (textproto.Header, string) {
	t.Helper()
	mr := gombox.NewReader(bytes.NewReader(data))
	msg, err := mr.NextMessage()
	require.NoError(t, err, "NextMessage")
	raw, err := io.ReadAll(msg)
	require.NoError(t, err, "ReadAll")
	sep := bytes.Index(raw, []byte("\r\n\r\n"))
	require.GreaterOrEqual(t, sep, 0, "no header/body separator found")
	h, err := textproto.ReadHeader(bufio.NewReader(
		bytes.NewReader(raw[:sep+4])))
	require.NoError(t, err, "ReadHeader")
	return h, string(raw[sep+4:])
}

func TestFormatBasicPatch(t *testing.T) {
	sub := baseSubmission()
	out := Format(sub)
	require.NotNil(t, out, "Format returned nil")

	h, body := parseMbox(t, out)

	assert.Equal(t, "Alice <alice@example.com>", h.Get("From"))
	assert.Equal(t, "[PATCH] fix something", h.Get("Subject"))
	assert.Equal(t, "42", h.Get("X-Patchwork-Id"))
	assert.Equal(t, `"Alice" <alice@example.com>`, h.Get("X-Patchwork-Submitter"))
	assert.Equal(t, "text/plain; charset=utf-8", h.Get("Content-Type"))
	assert.Equal(t, "8bit", h.Get("Content-Transfer-Encoding"))
	assert.Contains(t, body, "diff --git")
	assert.Contains(t, body, "Signed-off-by:")
	body = strings.ReplaceAll(body, "\r\n", "\n")
	assert.Contains(t, body, "---\n\ndiff --git")
}

func TestFormatParseable(t *testing.T) {
	sub := baseSubmission()
	out := Format(sub)

	mr := gombox.NewReader(bytes.NewReader(out))
	_, err := mr.NextMessage()
	assert.NoError(t, err, "output not parseable as mbox")
}

func TestFormatDelegate(t *testing.T) {
	sub := baseSubmission()
	sub.DelegateEmail = "bob@example.com"
	out := Format(sub)

	h, _ := parseMbox(t, out)
	assert.Equal(t, "bob@example.com", h.Get("X-Patchwork-Delegate"))
}

func TestFormatNoDelegateForCover(t *testing.T) {
	sub := baseSubmission()
	sub.Diff = ""
	sub.DelegateEmail = "bob@example.com"
	out := Format(sub)

	h, _ := parseMbox(t, out)
	assert.Empty(t, h.Get("X-Patchwork-Delegate"), "cover should not have X-Patchwork-Delegate")
}

func TestFormatDateAddedWhenMissing(t *testing.T) {
	sub := baseSubmission()
	sub.Headers = "From: Alice <alice@example.com>\nSubject: test\n"
	out := Format(sub)

	h, _ := parseMbox(t, out)
	assert.NotEmpty(t, h.Get("Date"), "Date header not added")
}

func TestFormatDatePreservedWhenPresent(t *testing.T) {
	sub := baseSubmission()
	sub.Headers = "From: Alice <alice@example.com>\n" +
		"Date: Mon, 01 Jan 2024 00:00:00 +0000\n"
	out := Format(sub)

	h, _ := parseMbox(t, out)
	assert.Contains(t, h.Get("Date"), "01 Jan 2024", "original Date not preserved")
}

func TestFormatListFromReplaced(t *testing.T) {
	sub := baseSubmission()
	sub.Headers = "From: list@lists.example.com\nSubject: test\n"
	sub.ListEmail = "list@lists.example.com"
	out := Format(sub)

	h, _ := parseMbox(t, out)
	assert.Equal(t, `"Alice" <alice@example.com>`, h.Get("From"))
	assert.Equal(t, "list@lists.example.com", h.Get("X-Patchwork-Original-From"))
}

func TestFormatFromNotReplacedWhenDifferent(t *testing.T) {
	sub := baseSubmission()
	sub.ListEmail = "list@lists.example.com"
	out := Format(sub)

	h, _ := parseMbox(t, out)
	assert.Equal(t, "Alice <alice@example.com>", h.Get("From"))
	assert.Empty(t, h.Get("X-Patchwork-Original-From"), "X-Patchwork-Original-From should be absent")
}

func TestFormatPostscript(t *testing.T) {
	sub := baseSubmission()
	sub.Content = "Signed-off-by: Alice <alice@example.com>\n---\nsome postscript notes\n"
	sub.Diff = "diff --git a/f b/f\n"
	out := Format(sub)

	_, body := parseMbox(t, out)
	body = strings.ReplaceAll(body, "\r\n", "\n")
	assert.Contains(t, body, "some postscript notes")
	assert.Contains(t, body, "Signed-off-by:")
}

func TestFormatCommentTags(t *testing.T) {
	sub := baseSubmission()
	sub.CommentContents = []string{
		"Looks good.\nReviewed-by: Bob <bob@example.com>\n",
		"Tested-by: Carol <carol@example.com>\n",
	}
	out := Format(sub)

	_, body := parseMbox(t, out)
	assert.Contains(t, body, "Reviewed-by: Bob <bob@example.com>")
	assert.Contains(t, body, "Tested-by: Carol <carol@example.com>")
}

func TestFormatEmptyHeaders(t *testing.T) {
	sub := baseSubmission()
	sub.Headers = ""
	out := Format(sub)
	require.NotNil(t, out, "Format returned nil with empty headers")

	h, _ := parseMbox(t, out)
	assert.Equal(t, "42", h.Get("X-Patchwork-Id"))
}

func TestFormatMboxEnvelope(t *testing.T) {
	sub := baseSubmission()
	out := Format(sub)

	first := string(out[:bytes.IndexByte(out, '\n')])
	assert.True(t, strings.HasPrefix(first, "From patchwork "), "mbox envelope = %q", first)
}

func TestFormatSignature(t *testing.T) {
	Version = "4.0.0"
	defer func() { Version = "" }()

	sub := baseSubmission()
	out := Format(sub)

	_, body := parseMbox(t, out)
	body = strings.ReplaceAll(body, "\r\n", "\n")
	assert.Contains(t, body, "-- \npatchwork 4.0.0\n")
}

func TestFormatNoSignatureWhenPresent(t *testing.T) {
	Version = "4.0.0"
	defer func() { Version = "" }()

	sub := baseSubmission()
	sub.Diff = "diff --git a/f b/f\n-- \nsome tool 1.0\n"
	out := Format(sub)

	_, body := parseMbox(t, out)
	body = strings.ReplaceAll(body, "\r\n", "\n")
	assert.NotContains(t, body, "patchwork 4.0.0")
}

func TestFormatNoSignatureWhenVersionEmpty(t *testing.T) {
	Version = ""

	sub := baseSubmission()
	out := Format(sub)

	_, body := parseMbox(t, out)
	body = strings.ReplaceAll(body, "\r\n", "\n")
	assert.NotContains(t, body, "-- \n")
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"[PATCH v2 1/3] net: fix buffer overflow", "patch-v2-1-3-net-fix-buffer-overflow"},
		{"simple-name", "simple-name"},
		{"UPPER.case", "upper.case"},
		{"", "patch"},
		{"!!!???", "patch"},
		{"file_name-1.0", "file_name-1.0"},
	}
	for _, tt := range tests {
		got := SanitizeFilename(tt.input)
		assert.Equal(t, tt.want, got)
	}
}
