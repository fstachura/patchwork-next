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
	if err != nil {
		t.Fatalf("NextMessage: %v", err)
	}
	raw, err := io.ReadAll(msg)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	sep := bytes.Index(raw, []byte("\r\n\r\n"))
	if sep < 0 {
		t.Fatal("no header/body separator found")
	}
	h, err := textproto.ReadHeader(bufio.NewReader(
		bytes.NewReader(raw[:sep+4])))
	if err != nil {
		t.Fatalf("ReadHeader: %v", err)
	}
	return h, string(raw[sep+4:])
}

func TestFormatBasicPatch(t *testing.T) {
	sub := baseSubmission()
	out := Format(sub)
	if out == nil {
		t.Fatal("Format returned nil")
	}

	h, body := parseMbox(t, out)

	if got := h.Get("From"); got != "Alice <alice@example.com>" {
		t.Errorf("From = %q", got)
	}
	if got := h.Get("Subject"); got != "[PATCH] fix something" {
		t.Errorf("Subject = %q", got)
	}
	if got := h.Get("X-Patchwork-Id"); got != "42" {
		t.Errorf("X-Patchwork-Id = %q", got)
	}
	if got := h.Get("X-Patchwork-Submitter"); got != `"Alice" <alice@example.com>` {
		t.Errorf("X-Patchwork-Submitter = %q", got)
	}
	if got := h.Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := h.Get("Content-Transfer-Encoding"); got != "8bit" {
		t.Errorf("Content-Transfer-Encoding = %q", got)
	}
	if !strings.Contains(body, "diff --git") {
		t.Error("body missing diff")
	}
	if !strings.Contains(body, "Signed-off-by:") {
		t.Error("body missing signed-off-by")
	}
	body = strings.ReplaceAll(body, "\r\n", "\n")
	if !strings.Contains(body, "---\n\ndiff --git") {
		t.Error("body missing --- separator before diff")
	}
}

func TestFormatCRLF(t *testing.T) {
	sub := baseSubmission()
	out := Format(sub)

	_, body := parseMbox(t, out)
	for i, line := range strings.Split(body, "\n") {
		if line == "" {
			continue
		}
		if !strings.HasSuffix(line, "\r") {
			t.Errorf("line %d not CRLF terminated: %q", i+1, line)
			break
		}
	}
}

func TestFormatDelegate(t *testing.T) {
	sub := baseSubmission()
	sub.DelegateEmail = "bob@example.com"
	out := Format(sub)

	h, _ := parseMbox(t, out)
	if got := h.Get("X-Patchwork-Delegate"); got != "bob@example.com" {
		t.Errorf("X-Patchwork-Delegate = %q", got)
	}
}

func TestFormatNoDelegateForCover(t *testing.T) {
	sub := baseSubmission()
	sub.Diff = ""
	sub.DelegateEmail = "bob@example.com"
	out := Format(sub)

	h, _ := parseMbox(t, out)
	if got := h.Get("X-Patchwork-Delegate"); got != "" {
		t.Errorf("cover should not have X-Patchwork-Delegate, got %q", got)
	}
}

func TestFormatDateAddedWhenMissing(t *testing.T) {
	sub := baseSubmission()
	sub.Headers = "From: Alice <alice@example.com>\nSubject: test\n"
	out := Format(sub)

	h, _ := parseMbox(t, out)
	if got := h.Get("Date"); got == "" {
		t.Error("Date header not added")
	}
}

func TestFormatDatePreservedWhenPresent(t *testing.T) {
	sub := baseSubmission()
	sub.Headers = "From: Alice <alice@example.com>\n" +
		"Date: Mon, 01 Jan 2024 00:00:00 +0000\n"
	out := Format(sub)

	h, _ := parseMbox(t, out)
	if got := h.Get("Date"); !strings.Contains(got, "01 Jan 2024") {
		t.Errorf("original Date not preserved: %q", got)
	}
}

func TestFormatListFromReplaced(t *testing.T) {
	sub := baseSubmission()
	sub.Headers = "From: list@lists.example.com\nSubject: test\n"
	sub.ListEmail = "list@lists.example.com"
	out := Format(sub)

	h, _ := parseMbox(t, out)
	if got := h.Get("From"); got != `"Alice" <alice@example.com>` {
		t.Errorf("From not replaced: %q", got)
	}
	if got := h.Get("X-Patchwork-Original-From"); got != "list@lists.example.com" {
		t.Errorf("X-Patchwork-Original-From = %q", got)
	}
}

func TestFormatFromNotReplacedWhenDifferent(t *testing.T) {
	sub := baseSubmission()
	sub.ListEmail = "list@lists.example.com"
	out := Format(sub)

	h, _ := parseMbox(t, out)
	if got := h.Get("From"); got != "Alice <alice@example.com>" {
		t.Errorf("From should not be replaced: %q", got)
	}
	if got := h.Get("X-Patchwork-Original-From"); got != "" {
		t.Errorf("X-Patchwork-Original-From should be absent, got %q", got)
	}
}

func TestFormatPostscript(t *testing.T) {
	sub := baseSubmission()
	sub.Content = "Signed-off-by: Alice <alice@example.com>\n---\nsome postscript notes\n"
	sub.Diff = "diff --git a/f b/f\n"
	out := Format(sub)

	_, body := parseMbox(t, out)
	body = strings.ReplaceAll(body, "\r\n", "\n")
	if !strings.Contains(body, "some postscript notes") {
		t.Errorf("body missing postscript content, got:\n%s", body)
	}
	if !strings.Contains(body, "Signed-off-by:") {
		t.Error("body missing Signed-off-by")
	}
}

func TestFormatCommentTags(t *testing.T) {
	sub := baseSubmission()
	sub.CommentContents = []string{
		"Looks good.\nReviewed-by: Bob <bob@example.com>\n",
		"Tested-by: Carol <carol@example.com>\n",
	}
	out := Format(sub)

	_, body := parseMbox(t, out)
	if !strings.Contains(body, "Reviewed-by: Bob <bob@example.com>") {
		t.Error("body missing Reviewed-by tag")
	}
	if !strings.Contains(body, "Tested-by: Carol <carol@example.com>") {
		t.Error("body missing Tested-by tag")
	}
}

func TestFormatEmptyHeaders(t *testing.T) {
	sub := baseSubmission()
	sub.Headers = ""
	out := Format(sub)
	if out == nil {
		t.Fatal("Format returned nil with empty headers")
	}

	h, _ := parseMbox(t, out)
	if got := h.Get("X-Patchwork-Id"); got != "42" {
		t.Errorf("X-Patchwork-Id = %q", got)
	}
}

func TestFormatMboxEnvelope(t *testing.T) {
	sub := baseSubmission()
	out := Format(sub)

	first := string(out[:bytes.IndexByte(out, '\n')])
	if !strings.HasPrefix(first, "From patchwork ") {
		t.Errorf("mbox envelope = %q", first)
	}
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
		if got != tt.want {
			t.Errorf("SanitizeFilename(%q) = %q, want %q",
				tt.input, got, tt.want)
		}
	}
}
