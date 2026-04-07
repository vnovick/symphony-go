// Package templates embeds workflow template files for use by itervox init.
package templates

import _ "embed"

// WorkflowLinear is the default WORKFLOW.md template for Linear-backed projects.
//
//go:embed workflow_linear.md
var WorkflowLinear []byte

// WorkflowGitHub is the default WORKFLOW.md template for GitHub-backed projects.
//
//go:embed workflow_github.md
var WorkflowGitHub []byte
