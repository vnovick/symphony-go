// Package templates embeds static markdown blocks appended by itervox init
// to every generated WORKFLOW.md. The dynamic portions of WORKFLOW.md are
// built inline in cmd/itervox/main.go; only fully-static sections live here.
package templates

import _ "embed"

// HumanInput is the static markdown block that instructs agents how and when
// to emit the <!-- itervox:needs-input --> sentinel. Appended to every
// generated WORKFLOW.md by `itervox init` so the contract reaches real
// projects instead of living only in this package.
//
//go:embed human_input.md
var HumanInput []byte
