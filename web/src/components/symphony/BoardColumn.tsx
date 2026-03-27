import { useRef } from 'react';
import { useDraggable, useDroppable } from '@dnd-kit/core';
import { CSS } from '@dnd-kit/utilities';
import IssueCard from './IssueCard';
import type { TrackerIssue } from '../../types/schemas';
import type { ProfileDef } from '../../types/schemas';

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
  isBeingDragged,
  shouldCollapse,
  onSelect,
  availableProfiles,
  profileDefs,
  runningBackendByIdentifier,
  onProfileChange,
  onDispatch,
}: {
  issue: TrackerIssue;
  /** True when THIS card is being dragged */
  isBeingDragged: boolean;
  /** True when the dragged card has left the source column (collapse the placeholder) */
  shouldCollapse: boolean;
  onSelect: (id: string) => void;
  availableProfiles?: string[];
  profileDefs?: Record<string, ProfileDef>;
  runningBackendByIdentifier?: Record<string, string>;
  onProfileChange?: (identifier: string, profile: string) => void;
  onDispatch?: (identifier: string) => void;
}) {
  const cardRef = useRef<HTMLDivElement>(null);
  const measuredHeight = useRef<number>(0);

  const { attributes, listeners, setNodeRef, transform, isDragging } = useDraggable({
    id: issue.identifier,
    data: { issue },
  });

  const style = transform && !isDragging
    ? { transform: CSS.Translate.toString(transform) }
    : undefined;

  // Measure card height before drag starts so placeholder matches exactly
  if (!isDragging && cardRef.current) {
    measuredHeight.current = cardRef.current.offsetHeight;
  }

  if (isDragging) {
    const h = measuredHeight.current || 72;
    return (
      <div
        ref={setNodeRef}
        {...attributes}
        {...listeners}
        className={`rounded-lg border-2 border-dashed border-theme-line-strong transition-all duration-300 ease-in-out overflow-hidden ${
          shouldCollapse
            ? 'max-h-0 opacity-0 my-0 border-0'
            : 'opacity-100'
        }`}
        style={shouldCollapse ? undefined : { height: h }}
      />
    );
  }

  return (
    <div
      ref={(node) => {
        setNodeRef(node);
        (cardRef as React.MutableRefObject<HTMLDivElement | null>).current = node;
      }}
      style={style}
      {...attributes}
      {...listeners}
    >
      <IssueCard
        issue={issue}
        isDragging={isBeingDragged}
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
  /** Display label — defaults to `state` if not provided */
  label?: string;
  issues: TrackerIssue[];
  isOver: boolean;
  /** Identifier of the card being dragged from this column (undefined if none) */
  draggingId?: string;
  /** True when the dragged card has moved to a different column */
  isCardOutside?: boolean;
  onSelect: (id: string) => void;
  availableProfiles?: string[];
  profileDefs?: Record<string, ProfileDef>;
  runningBackendByIdentifier?: Record<string, string>;
  onProfileChange?: (identifier: string, profile: string) => void;
  onDispatch?: (identifier: string) => void;
  /** Show (?) info button next to column header */
  onInfoClick?: () => void;
}

export default function BoardColumn({
  state,
  label,
  issues,
  isOver,
  draggingId,
  isCardOutside,
  onSelect,
  availableProfiles,
  profileDefs,
  runningBackendByIdentifier,
  onProfileChange,
  onDispatch,
  onInfoClick,
}: ColumnProps) {
  const { setNodeRef } = useDroppable({ id: state });
  const subtitle = columnSubtitle(label ?? state);

  return (
    <div
      ref={setNodeRef}
      className="relative flex w-[250px] min-w-[250px] flex-shrink-0 flex-col overflow-hidden rounded-[var(--radius-md)] max-h-[85vh] border border-theme-line bg-theme-bg-soft"
    >
      {/* Dark overlay when dropping */}
      <div
        className={`pointer-events-none absolute inset-0 z-10 rounded-[var(--radius-md)] transition-opacity duration-150 ${
          isOver ? 'opacity-100' : 'opacity-0'
        }`}
        style={{ background: 'rgba(0,0,0,0.25)' }}
      />

      {/* Column header */}
      <div className="flex flex-shrink-0 items-start justify-between gap-3 px-3 py-3">
        <div className="min-w-0">
          <span className="flex items-center gap-1.5">
            <h3 className="m-0 truncate text-xs font-semibold uppercase tracking-wide text-theme-text">
              {label ?? state}
            </h3>
            {onInfoClick && (
              <button
                onClick={(e) => { e.stopPropagation(); onInfoClick(); }}
                className="flex h-4 w-4 flex-shrink-0 items-center justify-center rounded-full text-[10px] text-theme-muted hover:text-theme-text transition-colors"
                title={`About ${label ?? state}`}
                aria-label={`Info about ${label ?? state}`}
              >
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <circle cx="12" cy="12" r="10" />
                  <path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3" />
                  <line x1="12" y1="17" x2="12.01" y2="17" />
                </svg>
              </button>
            )}
          </span>
          {subtitle && (
            <p className="mt-1 text-[11px] leading-snug text-theme-text-secondary">
              {subtitle}
            </p>
          )}
        </div>
        <span className="flex min-w-[28px] flex-shrink-0 items-center justify-center rounded-full px-2.5 py-1 text-xs font-semibold bg-theme-bg-elevated text-theme-text">
          {issues.length}
        </span>
      </div>

      {/* Issue cards */}
      <div className="flex-1 space-y-2 overflow-y-auto px-3 pb-3">
        {issues.map((issue) => (
          <DraggableCard
            key={issue.identifier}
            issue={issue}
            isBeingDragged={draggingId === issue.identifier}
            shouldCollapse={draggingId === issue.identifier && (isCardOutside ?? false)}
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
