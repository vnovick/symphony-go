import { useDraggable, useDroppable } from '@dnd-kit/core';
import { CSS } from '@dnd-kit/utilities';
import IssueCard from './IssueCard';
import type { TrackerIssue } from '../../types/symphony';
import type { ProfileDef } from '../../types/schemas';

// Lanes use bg-soft per prototype .lane spec
function stateTintVar(_state: string): string {
  return 'var(--bg-soft)';
}

// Descriptive subtitle for well-known state names
const COLUMN_SUBTITLES: Record<string, string> = {
  backlog: 'Unprioritized items',
  'to do': 'Ready to be picked up',
  todo: 'Ready to be picked up',
  'in progress': 'Active work',
  inprogress: 'Active work',
  'in review': 'Awaiting review',
  inreview: 'Awaiting review',
  review: 'Awaiting review',
  done: 'Completed this cycle',
  completed: 'Completed',
  cancelled: 'Closed issues',
  canceled: 'Closed issues',
  duplicate: 'Marked as duplicate',
};

function columnSubtitle(state: string): string | undefined {
  return COLUMN_SUBTITLES[state.toLowerCase().replace(/[-_]/g, ' ')];
}

function DraggableCard({
  issue,
  onSelect,
  availableProfiles,
  profileDefs,
  runningBackendByIdentifier,
  onProfileChange,
  onDispatch,
}: {
  issue: TrackerIssue;
  onSelect: (id: string) => void;
  availableProfiles?: string[];
  profileDefs?: Record<string, ProfileDef>;
  runningBackendByIdentifier?: Record<string, string>;
  onProfileChange?: (identifier: string, profile: string) => void;
  onDispatch?: (identifier: string) => void;
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
      <IssueCard
        issue={issue}
        isDragging={isDragging}
        onSelect={onSelect}
        availableProfiles={availableProfiles}
        profileDefs={profileDefs}
        runningBackend={runningBackendByIdentifier?.[issue.identifier]}
        onProfileChange={onProfileChange}
        onDispatch={onDispatch}
      />
    </div>
  );
}

interface ColumnProps {
  state: string;
  issues: TrackerIssue[];
  isOver: boolean;
  onSelect: (id: string) => void;
  availableProfiles?: string[];
  profileDefs?: Record<string, ProfileDef>;
  runningBackendByIdentifier?: Record<string, string>;
  onProfileChange?: (identifier: string, profile: string) => void;
  onDispatch?: (identifier: string) => void;
}

export default function BoardColumn({
  state,
  issues,
  isOver,
  onSelect,
  availableProfiles,
  profileDefs,
  runningBackendByIdentifier,
  onProfileChange,
  onDispatch,
}: ColumnProps) {
  const { setNodeRef } = useDroppable({ id: state });
  const subtitle = columnSubtitle(state);

  return (
    <div
      ref={setNodeRef}
      className="flex max-h-[85vh] flex-col overflow-hidden rounded-[var(--radius-md)] transition-all"
      style={{
        border: isOver ? '1px solid var(--accent)' : '1px solid var(--line)',
        background: stateTintVar(state),
      }}
    >
      {/* Column header — .lane-header spec */}
      <div className="flex flex-shrink-0 items-start justify-between gap-3 px-3 py-3">
        <div className="min-w-0">
          <h3
            className="m-0 truncate uppercase"
            style={{ fontSize: 12, fontWeight: 600, letterSpacing: '0.03em', color: 'var(--text)' }}
          >
            {state}
          </h3>
          {subtitle && (
            <p className="mt-1 leading-snug" style={{ fontSize: 11, color: 'var(--text-secondary)' }}>
              {subtitle}
            </p>
          )}
        </div>
        <span
          className="flex min-w-[28px] flex-shrink-0 items-center justify-center rounded-[var(--radius-full)] px-2.5 py-1"
          style={{ background: 'var(--bg-elevated)', fontSize: 12, fontWeight: 600, color: 'var(--text)' }}
        >
          {issues.length}
        </span>
      </div>

      {/* Issue cards — .lane-list spec: gap 8px, margin-top 10px */}
      <div className="flex-1 space-y-2 overflow-y-auto px-3 pb-3">
        {issues.map((issue) => (
          <DraggableCard
            key={issue.identifier}
            issue={issue}
            onSelect={onSelect}
            availableProfiles={availableProfiles}
            profileDefs={profileDefs}
            runningBackendByIdentifier={runningBackendByIdentifier}
            onProfileChange={onProfileChange}
            onDispatch={onDispatch}
          />
        ))}
      </div>
    </div>
  );
}
