package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/vnovick/itervox/internal/config"
	"github.com/vnovick/itervox/internal/server"
	"github.com/vnovick/itervox/internal/skills"
)

// initSkillsCache builds the in-memory skills cache and runs the first scan
// synchronously so the dashboard can serve a populated inventory on the very
// first /api/v1/skills/inventory request.
func (a *orchestratorAdapter) initSkillsCache() {
	a.skillsCache = skills.NewCache()
	if err := a.refreshSkillsInventory(context.Background()); err != nil {
		slog.Warn("skills: initial scan failed; serving empty inventory", "err", err)
	}
}

// projectAndHome resolves the (projectDir, homeDir) pair the scanners use.
// Project dir is derived from the WORKFLOW.md path; home dir falls back to
// "" if it can't be determined.
func (a *orchestratorAdapter) projectAndHome() (string, string) {
	projectDir := ""
	if a.workflowPath != "" {
		projectDir = filepath.Dir(a.workflowPath)
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = ""
	}
	return projectDir, homeDir
}

func (a *orchestratorAdapter) refreshSkillsInventory(ctx context.Context) error {
	if a.skillsCache == nil {
		a.skillsCache = skills.NewCache()
	}
	projectDir, homeDir := a.projectAndHome()
	scanFn := func() (*skills.Inventory, error) {
		return skills.Scan(projectDir, homeDir, skills.ScanOptions{})
	}
	tracked := skills.TrackedPathsFor(projectDir, homeDir)
	_ = ctx // reserved for future cancellable scans
	return a.skillsCache.Refresh(scanFn, tracked)
}

// --- server.SkillsClient implementation ---

func (a *orchestratorAdapter) Inventory() *skills.Inventory {
	if a.skillsCache == nil {
		return nil
	}
	return a.skillsCache.Get()
}

func (a *orchestratorAdapter) RefreshInventory(ctx context.Context) error {
	return a.refreshSkillsInventory(ctx)
}

func (a *orchestratorAdapter) Issues() []skills.InventoryIssue {
	inv := a.Inventory()
	if inv == nil {
		return nil
	}
	in := skills.AnalyzeInputs{
		Profiles: a.cfg.Agent.Profiles,
	}
	return skills.Analyze(inv, in)
}

// Analytics builds an AnalyticsSnapshot from the cached inventory + best-effort
// runtime evidence (T-100, T-102). Phase-2: runtime is parsed lazily on each
// call to keep the adapter simple. If this becomes hot, cache the runtime
// snapshot in a sibling struct alongside `skillsCache`.
func (a *orchestratorAdapter) Analytics() *skills.AnalyticsSnapshot {
	inv := a.Inventory()
	if inv == nil {
		return nil
	}
	_, homeDir := a.projectAndHome()
	logsDir := ""
	if homeDir != "" {
		logsDir = filepath.Join(homeDir, ".itervox", "logs")
	}
	claudeRT, _ := skills.ParseClaudeRuntime(logsDir, 25)
	codexRT, _ := skills.ParseCodexRuntime(homeDir, 25)
	merged := skills.MergeRuntimeSnapshots(claudeRT, codexRT)
	profiles := make([]string, 0, len(a.cfg.Agent.Profiles))
	for name := range a.cfg.Agent.Profiles {
		profiles = append(profiles, name)
	}
	return skills.BuildAnalytics(inv, merged, profiles)
}

// AnalyticsRecommendations runs the T-101 recommendation engine over the
// current Analytics + Inventory blend.
func (a *orchestratorAdapter) AnalyticsRecommendations() []skills.Recommendation {
	snap := a.Analytics()
	inv := a.Inventory()
	if snap == nil || inv == nil {
		return nil
	}
	return skills.RecommendAnalytics(snap, inv)
}

// ApplyFix implements server.SkillsClient.ApplyFix.
//
// Phase-1 supports a focused subset:
//   - "edit-yaml" with target "agent.profiles.<name>.enabled" → disables the
//     profile in WORKFLOW.md by calling UpsertProfile with Enabled=false.
//     Used by UNUSED_PROFILE (T-96).
//
// "remove-mcp" (DUPLICATE_MCP, T-95) is intentionally rejected here: editing
// the user's settings.json from the daemon is high-risk. The fix is exposed
// in the UI so an operator sees the recommendation and can edit the config
// manually. See `planning/deferred_290426.md` for the long form.
func (a *orchestratorAdapter) ApplyFix(_ context.Context, issueID string, fix skills.Fix) error {
	switch fix.Action {
	case "edit-yaml":
		const prefix = "agent.profiles."
		const suffix = ".enabled"
		if !strings.HasPrefix(fix.Target, prefix) || !strings.HasSuffix(fix.Target, suffix) {
			return fmt.Errorf("skills.ApplyFix: unsupported edit-yaml target %q for issue %s", fix.Target, issueID)
		}
		profileName := strings.TrimSuffix(strings.TrimPrefix(fix.Target, prefix), suffix)
		current, ok := a.cfg.Agent.Profiles[profileName]
		if !ok {
			return fmt.Errorf("skills.ApplyFix: profile %q does not exist", profileName)
		}
		def := serverProfileFromConfig(current)
		def.Enabled = false
		return a.UpsertProfile(profileName, def, profileName)
	case "remove-mcp":
		return fmt.Errorf("skills.ApplyFix: %s 'remove-mcp' is intentionally deferred — edit settings.json by hand; see planning/deferred_290426.md", issueID)
	default:
		return fmt.Errorf("skills.ApplyFix: unknown action %q for issue %s", fix.Action, issueID)
	}
}

func serverProfileFromConfig(p config.AgentProfile) server.ProfileDef {
	enabled := true
	if p.Enabled != nil {
		enabled = *p.Enabled
	}
	return server.ProfileDef{
		Command:          p.Command,
		Prompt:           p.Prompt,
		Backend:          p.Backend,
		Enabled:          enabled,
		AllowedActions:   p.AllowedActions,
		CreateIssueState: p.CreateIssueState,
	}
}
