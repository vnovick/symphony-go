import { useState, useMemo, useCallback } from 'react';
import {
  DndContext,
  DragOverlay,
  PointerSensor,
  useSensor,
  useSensors,
  useDraggable,
  useDroppable,
  type DragEndEvent,
  type DragStartEvent,
  type DragOverEvent,
} from '@dnd-kit/core';
import { CSS } from '@dnd-kit/utilities';
import IssueCard from './IssueCard';
import type { TrackerIssue } from '../../types/symphony';

// Sentinel droppable ID for the "Unassigned" column — dnd-kit requires a truthy string ID.
const UNASSIGNED = '__unassigned__';

function DraggableQueueCard({
  issue,
  onSelect,
}: {
  issue: TrackerIssue;
  onSelect: (id: string) => void;
}) {
  const { attributes, listeners, setNodeRef, transform, isDragging } = useDraggable({
    id: issue.identifier,
    data: { issue },
  });
  const style = transform
    ? { transform: CSS.Translate.toString(transform), zIndex: isDragging ? 999 : undefined }
    : undefined;
  return (
    <div ref={setNodeRef} style={style} {...attributes} {...listeners}>
      <IssueCard issue={issue} isDragging={isDragging} onSelect={onSelect} />
    </div>
  );
}

function AgentColumn({
  droppableId,
  label,
  isUnassigned,
  issues,
  isOver,
  onSelect,
}: {
  droppableId: string;
  label: string;
  isUnassigned: boolean;
  issues: TrackerIssue[];
  isOver: boolean;
  onSelect: (id: string) => void;
}) {
  const { setNodeRef } = useDroppable({ id: droppableId });

  return (
    <div
      ref={setNodeRef}
      className={`flex w-64 flex-shrink-0 flex-col overflow-hidden rounded-2xl border shadow-xs transition-all ${
        isOver
          ? 'border-brand-300 bg-brand-50/40 dark:bg-brand-900/10 shadow-md'
          : 'border-gray-200 bg-gray-50/60 dark:border-gray-800 dark:bg-white/[0.02]'
      }`}
    >
      <div className="flex flex-shrink-0 items-center justify-between px-3 py-2.5">
        <div className="flex items-center gap-1.5">
          <span className={`text-xs ${isUnassigned ? 'text-gray-400' : 'text-purple-500'}`}>
            {isUnassigned ? '○' : '◈'}
          </span>
          <span className="truncate text-xs font-semibold tracking-wide text-gray-600 uppercase dark:text-gray-400">
            {label}
          </span>
        </div>
        <span className="ml-2 flex h-5 min-w-[20px] flex-shrink-0 items-center justify-center rounded-full border border-gray-200 bg-white px-1.5 text-[10px] font-bold text-gray-500 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-400">
          {issues.length}
        </span>
      </div>
      <div className="max-h-[calc(100vh-300px)] min-h-[80px] flex-1 space-y-1.5 overflow-y-auto px-2 pb-2">
        {issues.map((issue) => (
          <DraggableQueueCard key={issue.identifier} issue={issue} onSelect={onSelect} />
        ))}
        {issues.length === 0 && (
          <div className="flex h-16 items-center justify-center rounded-xl border-2 border-dashed border-gray-200 text-xs text-gray-300 dark:border-gray-800 dark:text-gray-600">
            Drop here
          </div>
        )}
      </div>
    </div>
  );
}

interface AgentQueueViewProps {
  issues: TrackerIssue[];
  backlogStates: string[];
  availableProfiles: string[];
  onProfileChange: (identifier: string, profile: string) => void;
  onSelect: (id: string) => void;
}

export default function AgentQueueView({
  issues,
  backlogStates,
  availableProfiles,
  onProfileChange,
  onSelect,
}: AgentQueueViewProps) {
  const [activeIssue, setActiveIssue] = useState<TrackerIssue | null>(null);
  const [overId, setOverId] = useState<string | null>(null);

  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 5 } }));

  const backlogSet = useMemo(() => new Set(backlogStates), [backlogStates]);

  // Group backlog issues by profile. Issues assigned to unknown profiles fall to unassigned.
  const columns = useMemo<Array<{ id: string; label: string; issues: TrackerIssue[] }>>(() => {
    const unassigned: TrackerIssue[] = [];
    const byProfile = new Map<string, TrackerIssue[]>(availableProfiles.map((p) => [p, []]));
    for (const issue of issues) {
      if (!backlogSet.has(issue.state)) continue;
      const p = issue.agentProfile;
      if (p && byProfile.has(p)) {
        (byProfile.get(p) as TrackerIssue[]).push(issue);
      } else {
        unassigned.push(issue);
      }
    }
    return [
      { id: UNASSIGNED, label: 'Unassigned', issues: unassigned },
      ...availableProfiles.map((p) => ({ id: p, label: p, issues: byProfile.get(p) ?? [] })),
    ];
  }, [issues, backlogSet, availableProfiles]);

  const onDragStart = useCallback((e: DragStartEvent) => {
    setActiveIssue((e.active.data.current as { issue: TrackerIssue }).issue);
  }, []);

  const onDragOver = useCallback((e: DragOverEvent) => {
    setOverId(e.over ? String(e.over.id) : null);
  }, []);

  const onDragEnd = useCallback(
    (e: DragEndEvent) => {
      setActiveIssue(null);
      setOverId(null);
      if (!e.over) return;
      const droppedOn = String(e.over.id);
      // Map the sentinel back to an empty string (clears the profile assignment).
      const newProfile = droppedOn === UNASSIGNED ? '' : droppedOn;
      const currentProfile =
        (e.active.data.current as { issue: TrackerIssue }).issue.agentProfile ?? '';
      if (newProfile !== currentProfile) {
        onProfileChange(String(e.active.id), newProfile);
      }
    },
    [onProfileChange],
  );

  const totalBacklog = columns.reduce((sum, col) => sum + col.issues.length, 0);

  if (totalBacklog === 0) {
    return (
      <div className="rounded-2xl border border-gray-200 bg-white px-6 py-12 text-center text-sm text-gray-400 dark:border-gray-800 dark:bg-white/[0.03]">
        No backlog issues to route
      </div>
    );
  }

  return (
    <DndContext
      sensors={sensors}
      onDragStart={onDragStart}
      onDragOver={onDragOver}
      onDragEnd={onDragEnd}
    >
      <div className="flex gap-4 overflow-x-auto pb-4">
        {columns.map((col) => (
          <AgentColumn
            key={col.id}
            droppableId={col.id}
            label={col.label}
            isUnassigned={col.id === UNASSIGNED}
            issues={col.issues}
            isOver={overId === col.id}
            onSelect={onSelect}
          />
        ))}
      </div>
      <DragOverlay>
        {activeIssue && <IssueCard issue={activeIssue} isDragging onSelect={() => {}} />}
      </DragOverlay>
    </DndContext>
  );
}
