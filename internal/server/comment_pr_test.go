package server

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ValidateCommentPRRequest must reject empty submissions, file-less findings,
// negative line numbers, missing bodies, and malformed severity values.
func TestValidateCommentPRRequest_Rejects(t *testing.T) {
	cases := []struct {
		name string
		req  CommentPRRequest
		want string
	}{
		{
			name: "empty submission",
			req:  CommentPRRequest{},
			want: "summary or at least one finding",
		},
		{
			name: "missing path",
			req: CommentPRRequest{
				Findings: []CommentPRFinding{{Line: 1, Severity: "error", Body: "x"}},
			},
			want: "path is required",
		},
		{
			name: "negative line",
			req: CommentPRRequest{
				Findings: []CommentPRFinding{{Path: "a.go", Line: -1, Severity: "info", Body: "x"}},
			},
			want: "line must be >= 0",
		},
		{
			name: "missing body",
			req: CommentPRRequest{
				Findings: []CommentPRFinding{{Path: "a.go", Line: 1, Severity: "info", Body: "  "}},
			},
			want: "body is required",
		},
		{
			name: "missing severity",
			req: CommentPRRequest{
				Findings: []CommentPRFinding{{Path: "a.go", Line: 1, Body: "x"}},
			},
			want: "severity is required",
		},
		{
			name: "bad severity",
			req: CommentPRRequest{
				Findings: []CommentPRFinding{{Path: "a.go", Line: 1, Severity: "panic", Body: "x"}},
			},
			want: "severity must be info, warning, or error",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateCommentPRRequest(tc.req)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

// ValidateCommentPRRequest must accept summary-only and finding-only shapes,
// and accept severity in any case.
func TestValidateCommentPRRequest_Accepts(t *testing.T) {
	cases := []CommentPRRequest{
		{Summary: "Just a summary, no findings."},
		{
			Findings: []CommentPRFinding{
				{Path: "x.go", Line: 1, Severity: "ERROR", Body: "case-insensitive ok"},
				{Path: "x.go", Line: 0, Severity: "Warning", Body: "file-level ok"},
			},
		},
	}
	for i, req := range cases {
		t.Run("", func(t *testing.T) {
			_ = i
			assert.NoError(t, ValidateCommentPRRequest(req))
		})
	}
}

// RenderCommentPRMarkdown deterministically sorts errors → warnings → info,
// then by path, then by line. Stability across calls is the contract.
func TestRenderCommentPRMarkdown_DeterministicOrder(t *testing.T) {
	req := CommentPRRequest{
		Summary: "Two issues found across two files.",
		Findings: []CommentPRFinding{
			{Path: "z.go", Line: 5, Severity: "info", Body: "trivial"},
			{Path: "a.go", Line: 9, Severity: "error", Body: "nil deref"},
			{Path: "a.go", Line: 1, Severity: "error", Body: "missing import"},
			{Path: "m.go", Line: 0, Severity: "warning", Body: "TODO without ticket"},
		},
	}
	out := RenderCommentPRMarkdown(req)

	// Header + summary first, then Findings section.
	assert.Contains(t, out, "## 🤖 Itervox review")
	assert.Contains(t, out, "Two issues found across two files.")
	assert.Contains(t, out, "### Findings (4)")

	// Errors first (a.go:1 before a.go:9), then warning, then info.
	idxErr1 := strings.Index(out, "missing import")
	idxErr9 := strings.Index(out, "nil deref")
	idxWarn := strings.Index(out, "TODO without ticket")
	idxInfo := strings.Index(out, "trivial")
	require.NotEqual(t, -1, idxErr1)
	require.NotEqual(t, -1, idxErr9)
	require.NotEqual(t, -1, idxWarn)
	require.NotEqual(t, -1, idxInfo)
	assert.Less(t, idxErr1, idxErr9, "a.go:1 (error) before a.go:9 (error): path then line")
	assert.Less(t, idxErr9, idxWarn, "errors before warnings")
	assert.Less(t, idxWarn, idxInfo, "warnings before info")

	// File-level finding (line=0) renders without ":0" suffix.
	assert.Contains(t, out, "`m.go`")
	assert.NotContains(t, out, "m.go:0")

	// Same render twice → byte-for-byte identical (idempotency contract).
	out2 := RenderCommentPRMarkdown(req)
	assert.Equal(t, out, out2)
}

// Summary-only requests render without a Findings section.
func TestRenderCommentPRMarkdown_SummaryOnly(t *testing.T) {
	out := RenderCommentPRMarkdown(CommentPRRequest{Summary: "All clean."})
	assert.Contains(t, out, "All clean.")
	assert.NotContains(t, out, "### Findings")
}
