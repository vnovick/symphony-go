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
import IssueCard from '../../../components/symphony/IssueCard';
import BoardColumn from '../../../components/symphony/BoardColumn';
import { useSymphonyStore } from '../../../store/symphonyStore';
import type { TrackerIssue, RunningRow } from '../../../types/schemas';

const EMPTY_RUNNING: RunningRow[] = [];
const EMPTY_STATES: string[] = [];

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
    backlogStates,
    activeStates,
    completionState,
    terminalStates,
  } = useSymphonyStore(
    useShallow((s) => ({
      snapshotLoaded: s.snapshot !== null,
      profileDefs: s.snapshot?.profileDefs,
      running: s.snapshot?.running ?? EMPTY_RUNNING,
      backlogStates: s.snapshot?.backlogStates ?? EMPTY_STATES,
      activeStates: s.snapshot?.activeStates ?? EMPTY_STATES,
      completionState: s.snapshot?.completionState ?? '',
      terminalStates: s.snapshot?.terminalStates ?? EMPTY_STATES,
    })),
  );
  const [activeIssue, setActiveIssue] = useState<TrackerIssue | null>(null);
  const [overId, setOverId] = useState<string | null>(null);

  const runningBackendByIdentifier = useMemo(() => {
    const map: Record<string, string> = {};
    for (const r of running) {
      if (r.backend) map[r.identifier] = r.backend;
    }
    return map;
  }, [running]);

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
    useSensor(TouchSensor, { activationConstraint: { delay: 250, tolerance: 5 } }),
    useSensor(KeyboardSensor),
  );

  const backlogStateSet = useMemo(
    () => new Set(backlogStates),
    [backlogStates],
  );
  const firstActiveState = activeStates[0] ?? '';

  const handleDispatch = useCallback(
    (identifier: string) => {
      if (firstActiveState) void onStateChange(identifier, firstActiveState);
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
    void onStateChange(identifier, newState);
  };

  if (!snapshotLoaded) {
    return (
      <div className="py-8 text-center text-sm text-theme-muted">
        Loading…
      </div>
    );
  }

  return (
    <DndContext
      sensors={sensors}
      onDragStart={handleDragStart}
      onDragOver={handleDragOver}
      onDragEnd={handleDragEnd}
    >
      {/* Horizontal scroll — same as Linear on all screen sizes */}
      <div className="flex gap-3 overflow-x-auto pb-2 min-h-[200px]">
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
