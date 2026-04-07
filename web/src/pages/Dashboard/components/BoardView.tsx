import { useState, useMemo, useCallback } from 'react';
import {
  DndContext,
  DragOverlay,
  KeyboardSensor,
  PointerSensor,
  TouchSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragStartEvent,
  type DragOverEvent,
} from '@dnd-kit/core';
import { useShallow } from 'zustand/react/shallow';
import IssueCard from '../../../components/itervox/IssueCard';
import BoardColumn from '../../../components/itervox/BoardColumn';
import { useItervoxStore } from '../../../store/itervoxStore';
import type { TrackerIssue } from '../../../types/schemas';
import { EMPTY_RUNNING, EMPTY_HISTORY, EMPTY_STATES } from '../../../utils/constants';

interface BoardViewProps {
  issues: TrackerIssue[];
  onSelect: (id: string) => void;
  onStateChange: (identifier: string, newState: string) => void;
  availableProfiles: string[];
  onProfileChange: (identifier: string, profile: string) => void;
}

export function BoardView({
  issues,
  onSelect,
  onStateChange,
  availableProfiles,
  onProfileChange,
}: BoardViewProps) {
  const {
    snapshotLoaded,
    profileDefs,
    running,
    history: runHistory,
    defaultBackend,
    backlogStates,
    activeStates,
    completionState,
    terminalStates,
  } = useItervoxStore(
    useShallow((s) => ({
      snapshotLoaded: s.snapshot !== null,
      profileDefs: s.snapshot?.profileDefs,
      running: s.snapshot?.running ?? EMPTY_RUNNING,
      history: s.snapshot?.history ?? EMPTY_HISTORY,
      defaultBackend: s.snapshot?.defaultBackend,
      backlogStates: s.snapshot?.backlogStates ?? EMPTY_STATES,
      activeStates: s.snapshot?.activeStates ?? EMPTY_STATES,
      completionState: s.snapshot?.completionState ?? '',
      terminalStates: s.snapshot?.terminalStates ?? EMPTY_STATES,
    })),
  );
  const [activeIssue, setActiveIssue] = useState<TrackerIssue | null>(null);
  const [overId, setOverId] = useState<string | null>(null);

  const backlogStateSet = useMemo(() => new Set(backlogStates), [backlogStates]);

  const runningBackendByIdentifier = useMemo(() => {
    const map: Record<string, string> = {};
    // History: only include non-backlog issues so that issues moved back to
    // Todo/Backlog show the profile/default badge, not a stale history backend.
    const backlogIdentifiers = new Set(
      issues.filter((i) => backlogStateSet.has(i.state)).map((i) => i.identifier),
    );
    for (const h of runHistory) {
      if (h.backend && !backlogIdentifiers.has(h.identifier)) map[h.identifier] = h.backend;
    }
    // Running entries always take priority.
    for (const r of running) {
      if (r.backend) map[r.identifier] = r.backend;
    }
    return map;
  }, [running, runHistory, issues, backlogStateSet]);

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
    useSensor(TouchSensor, { activationConstraint: { delay: 250, tolerance: 5 } }),
    useSensor(KeyboardSensor),
  );

  const firstActiveState = activeStates[0] ?? '';

  const handleDispatch = useCallback(
    (identifier: string) => {
      if (firstActiveState) onStateChange(identifier, firstActiveState);
    },
    [onStateChange, firstActiveState],
  );

  const columnNames = useMemo(() => {
    const completion = completionState ? [completionState] : [];
    const seen = new Set<string>();
    const cols: string[] = [];
    for (const s of [...backlogStates, ...activeStates, ...completion, ...terminalStates]) {
      if (!seen.has(s)) {
        seen.add(s);
        cols.push(s);
      }
    }
    if (cols.length === 0) {
      return Array.from(new Set(issues.map((i) => i.state)));
    }
    return cols;
  }, [backlogStates, activeStates, completionState, terminalStates, issues]);

  const columns = useMemo(() => {
    return columnNames.map((state) => {
      const colIssues = issues.filter((i) => i.state === state);
      return [state, colIssues] as const;
    });
  }, [columnNames, issues]);

  const handleDragStart = (event: DragStartEvent) => {
    const issue = issues.find((i) => i.identifier === event.active.id);
    setActiveIssue(issue ?? null);
  };

  const handleDragOver = (event: DragOverEvent) => {
    setOverId(event.over?.id ? String(event.over.id) : null);
  };

  const handleDragEnd = (event: DragEndEvent) => {
    setActiveIssue(null);
    setOverId(null);
    const { active, over } = event;
    if (!over) return;
    const identifier = String(active.id);
    const newState = String(over.id);
    const current = issues.find((i) => i.identifier === identifier);
    if (!current || current.state === newState) return;
    onStateChange(identifier, newState);
  };

  if (!snapshotLoaded) {
    return <div className="text-theme-muted py-8 text-center text-sm">Loading…</div>;
  }

  return (
    <DndContext
      sensors={sensors}
      onDragStart={handleDragStart}
      onDragOver={handleDragOver}
      onDragEnd={handleDragEnd}
    >
      {/* Horizontal scroll — same as Linear on all screen sizes */}
      <div className="flex min-h-[200px] gap-3 overflow-x-auto pb-2">
        {columns.map(([state, colIssues]) => (
          <BoardColumn
            key={state}
            state={state}
            issues={colIssues}
            isOver={overId === state}
            draggingId={activeIssue?.state === state ? activeIssue.identifier : undefined}
            isCardOutside={activeIssue?.state === state && overId !== null && overId !== state}
            onSelect={onSelect}
            availableProfiles={availableProfiles}
            profileDefs={profileDefs}
            runningBackendByIdentifier={runningBackendByIdentifier}
            defaultBackend={defaultBackend}
            onProfileChange={onProfileChange}
            onDispatch={backlogStateSet.has(state) ? handleDispatch : undefined}
          />
        ))}
      </div>
      <DragOverlay dropAnimation={null}>
        {activeIssue && (
          <div className="w-[250px]">
            <IssueCard issue={activeIssue} isDragging onSelect={() => {}} />
          </div>
        )}
      </DragOverlay>
    </DndContext>
  );
}
