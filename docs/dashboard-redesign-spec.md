# Dashboard Redesign Spec

## Objective

Redesign the Symphony dashboard so it feels like a premium operations product rather than an internal admin console.

The redesign should improve:

- visual hierarchy
- live-operations readability
- perceived product quality
- clarity across Claude and Codex backends
- decision speed for a developer or tech lead checking the system

## Core Product Thesis

The dashboard should read as a control deck.

Users should be able to answer these questions within 5 to 10 seconds:

1. Is the system healthy right now?
2. What is actively running?
3. Where is work blocked, paused, or overloaded?
4. Which issues need intervention?
5. How much capacity do I have left?

## Design Direction

Recommended direction: `Control Room`.

Characteristics:

- dark, atmospheric hero band over a lighter workspace
- strong top-level metrics with clear severity states
- live sessions treated as the primary surface
- board and queues presented as work lanes, not generic tables
- restrained motion only on truly live elements

Alternative directions worth considering:

- `Project Radar`: calmer, more planning-centric, issue board first
- `Executive Briefing`: summary first, operations detail collapsed

## Information Hierarchy

### Level 1: System Pulse

Top band showing:

- live status
- agent mode
- backend mix
- worker capacity
- retry / blocked pressure
- last orchestration event

### Level 2: Active Work

Primary workspace row:

- live missions / running sessions
- health rail with rate limit, delegation mix, and hotspots

### Level 3: Workstream Lanes

Issue lanes grouped by operational intent:

- up next
- deep work
- review queue
- paused / blocked

### Level 4: Activity Narrative

Condensed timeline or log feed:

- recent handoffs
- completed work
- new blockers

## UI Modules

### 1. Hero / Pulse Deck

Replace the current title + status strip with a stronger hero module.

Contents:

- page title and short descriptor
- live state badge
- orchestration summary
- main capacity meter
- key metric cards

### 2. Mission Stack

Replace the current dense running table with card-rows.

Each mission row should show:

- issue identity and title
- backend and profile
- active subagent count
- latest event snippet
- elapsed time
- quick actions

### 3. Health Rail

Right-side companion rail for:

- API headroom
- retries
- blocked issues
- worker saturation
- backend distribution

### 4. Priority Lanes

Recast the issue area as status-aware lanes with stronger headers and counts.

Each lane should have:

- semantic color
- short summary line
- compact issue cards
- visible risk or waiting state

### 5. Narrative Feed

Short timeline for notable orchestration events.

Examples:

- reviewer spawned
- retry backoff started
- PR opened
- rate limit pressure increased

## Content and Copy Changes

- Replace Claude-only wording with backend-neutral wording.
- Keep "live" language, but make it more specific and less decorative.
- Prefer operational language: `missions`, `lanes`, `headroom`, `handoffs`, `capacity`.
- If `Simphony` is accidental, normalize all product naming to `Symphony`.

## Visual System

### Palette

- ink / slate base
- cobalt primary accent
- mint for healthy live states
- amber for warnings
- coral for blockers

### Surfaces

- atmospheric hero surface with gradients and subtle grid texture
- lighter working surfaces below
- clearer elevation separation between major modules

### Typography

- strong title weight
- tighter uppercase labels for control-room metadata
- mono only for IDs, timing, and logs

### Motion

- soft pulse on live indicators
- small number tick animations
- smooth hover and filter transitions
- no constant ambient animation outside the hero

## Interaction Principles

- show more by default, hide less critical detail in rails and drawers
- keep filters and mode switches near the content they affect
- make actions obvious but secondary to state comprehension
- maintain legibility on laptop widths without relying on giant tables

## Implementation Phases

### Phase 1

- redesign hero
- redesign running sessions
- redesign health rail

### Phase 2

- redesign issue lanes
- refine issue detail modal to match the new language

### Phase 3

- add motion polish
- add saved layout preferences if needed

## Success Criteria

- dashboard feels differentiated from default SaaS admin layouts
- users can identify pressure points without opening logs
- visual density stays high without becoming noisy
- backend-agnostic orchestration is obvious from the UI

## Prototype Notes

The first prototype should optimize for:

- stronger first impression
- better hierarchy than the current dashboard
- a more premium live-ops mood
- easy comparison between at least two design directions
