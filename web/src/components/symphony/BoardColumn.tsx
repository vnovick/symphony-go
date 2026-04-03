import { useRef } from 'react';
import { useDraggable, useDroppable } from '@dnd-kit/core';
import { CSS } from '@dnd-kit/utilities';
import IssueCard from './IssueCard';
import type { TrackerIssue } from '../../types/schemas';
import type { ProfileDef } from '../../types/schemas';
import {
  profileColor,
  profileInitials,
  UNASSIGNED_COLOR,
  type ProfileColor,
} from '../../utils/profileColors';
import {
  backendLabel,
  backendBadgeClass,
} from '../../pages/Settings/profiles/ProfileEditorFields';
import { commandToBackend, commandToModel, modelLabel } from '../../pages/Settings/profileCommands';

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
  defaultBackend,
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
  defaultBackend?: string;
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
        defaultBackend={defaultBackend}
        onProfileChange={onProfileChange}
        onDispatch={onDispatch}
      />
    </div>
  );
}

// ─── Rich agent header (used in Agent Queue view) ─────────────────────

function AgentColumnHeader({
  label,
  profileDef,
  color,
  issueCount,
  onHeaderClick,
}: {
  label: string;
  profileDef?: ProfileDef;
  color: ProfileColor;
  issueCount: number;
  onHeaderClick?: () => void;
}) {
  const initials = profileInitials(label);
  const inferredBackend = profileDef
    ? commandToBackend(profileDef.command, profileDef.backend)
    : 'claude';
  const model = profileDef ? commandToModel(profileDef.command) : '';
  const modelDisplay = model ? modelLabel(inferredBackend, model) : '';
  const promptSnippet = profileDef?.prompt
    ? profileDef.prompt.length > 100
      ? profileDef.prompt.slice(0, 100) + '…'
      : profileDef.prompt
    : null;

  return (
    <div
      className={`flex-shrink-0 ${onHeaderClick ? 'cursor-pointer' : ''}`}
      onClick={onHeaderClick}
      role={onHeaderClick ? 'button' : undefined}
      tabIndex={onHeaderClick ? 0 : undefined}
      onKeyDown={onHeaderClick ? (e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onHeaderClick(); } } : undefined}
    >
      {/* Colored top edge */}
      <div
        className="h-[3px] rounded-t-[var(--radius-md)]"
        style={{ background: `linear-gradient(90deg, ${color.accent}, ${color.accent}44)`, opacity: 0.8 }}
      />

      <div className="px-3 pt-3 pb-2">
        <div className="flex items-start gap-2.5">
          {/* Avatar */}
          <div
            className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-[10px] text-xs font-bold text-white relative overflow-hidden"
            style={{ background: color.gradient }}
          >
            <span className="relative z-10">{initials}</span>
            <div className="absolute inset-0 bg-gradient-to-br from-white/15 to-transparent rounded-inherit" />
          </div>

          {/* Meta */}
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-1.5">
              <h3 className="m-0 truncate text-xs font-semibold text-theme-text">
                {label}
              </h3>
              {profileDef && (
                <>
                <span
                  className={`flex-shrink-0 rounded-full px-1.5 py-0.5 text-[9px] font-semibold uppercase tracking-wide ${backendBadgeClass(inferredBackend)}`}
                >
                  {backendLabel(inferredBackend)}
                </span>
                {modelDisplay && (
                  <span className="flex-shrink-0 rounded-full px-1.5 py-0.5 text-[9px] font-medium text-theme-text-secondary bg-theme-bg-soft">
                    {modelDisplay}
                  </span>
                )}
                </>
              )}
            </div>
            {promptSnippet && (
              <p className="mt-1 text-[11px] leading-snug text-theme-text-secondary line-clamp-2">
                {promptSnippet}
              </p>
            )}
          </div>

          {/* Count badge */}
          <span
            className="flex min-w-[28px] flex-shrink-0 items-center justify-center rounded-full px-2.5 py-1 text-xs font-semibold"
            style={{ background: color.bg, color: color.accent, border: `1px solid ${color.accent}22` }}
          >
            {issueCount}
          </span>
        </div>
      </div>
    </div>
  );
}

// ─── Unassigned column header ─────────────────────────────────────────

function UnassignedColumnHeader({ issueCount }: { issueCount: number }) {
  return (
    <div className="flex-shrink-0">
      <div
        className="h-[3px] rounded-t-[var(--radius-md)]"
        style={{ background: `linear-gradient(90deg, ${UNASSIGNED_COLOR.accent}, ${UNASSIGNED_COLOR.accent}44)`, opacity: 0.5 }}
      />
      <div className="px-3 pt-3 pb-2">
        <div className="flex items-start gap-2.5">
          <div
            className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-[10px] text-sm text-theme-muted"
            style={{ background: UNASSIGNED_COLOR.gradient }}
          >
            ?
          </div>
          <div className="min-w-0 flex-1">
            <h3 className="m-0 truncate text-xs font-semibold text-theme-text">
              Unassigned
            </h3>
            <p className="mt-1 text-[11px] leading-snug text-theme-text-secondary">
              Drag to an agent to assign
            </p>
          </div>
          <span className="flex min-w-[28px] flex-shrink-0 items-center justify-center rounded-full px-2.5 py-1 text-xs font-semibold bg-theme-bg-elevated text-theme-text">
            {issueCount}
          </span>
        </div>
      </div>
    </div>
  );
}

// ─── Default state-based header (Board view) ──────────────────────────

function DefaultColumnHeader({
  state,
  label,
  issueCount,
  onInfoClick,
}: {
  state: string;
  label: string;
  issueCount: number;
  onInfoClick?: () => void;
}) {
  const subtitle = columnSubtitle(label ?? state);
  return (
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
        {issueCount}
      </span>
    </div>
  );
}

// ─── Main BoardColumn ─────────────────────────────────────────────────

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
  defaultBackend?: string;
  onProfileChange?: (identifier: string, profile: string) => void;
  onDispatch?: (identifier: string) => void;
  /** Show (?) info button next to column header (Board view) */
  onInfoClick?: () => void;
  /** Click handler for the rich agent header (Agent Queue view) */
  onHeaderClick?: () => void;
  /** Whether this column represents the "unassigned" bucket */
  isUnassigned?: boolean;
  /** ProfileDef for this specific column's agent (Agent Queue view) */
  columnProfileDef?: ProfileDef;
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
  defaultBackend,
  onProfileChange,
  onDispatch,
  onInfoClick,
  onHeaderClick,
  isUnassigned,
  columnProfileDef,
}: ColumnProps) {
  const { setNodeRef } = useDroppable({ id: state });

  // Determine which header to render:
  // - isUnassigned === true → UnassignedColumnHeader
  // - columnProfileDef or onHeaderClick → AgentColumnHeader (rich)
  // - default → DefaultColumnHeader (state-based)
  const isAgentView = isUnassigned !== undefined;
  const color = isAgentView && !isUnassigned ? profileColor(label ?? state) : null;

  return (
    <div
      ref={setNodeRef}
      className="relative flex w-[280px] min-w-[280px] flex-shrink-0 flex-col overflow-hidden rounded-[var(--radius-md)] max-h-[85vh] border border-theme-line bg-theme-bg-soft transition-colors duration-200"
      style={
        color && isOver
          ? { borderColor: `${color.accent}44` }
          : undefined
      }
    >
      {/* Dark overlay when dropping */}
      <div
        className={`pointer-events-none absolute inset-0 z-10 rounded-[var(--radius-md)] transition-opacity duration-150 ${
          isOver ? 'opacity-100' : 'opacity-0'
        }`}
        style={{ background: 'rgba(0,0,0,0.25)' }}
      />

      {/* Column header */}
      {isUnassigned ? (
        <UnassignedColumnHeader issueCount={issues.length} />
      ) : isAgentView && color ? (
        <AgentColumnHeader
          label={label ?? state}
          profileDef={columnProfileDef}
          color={color}
          issueCount={issues.length}
          onHeaderClick={onHeaderClick}
        />
      ) : (
        <DefaultColumnHeader
          state={state}
          label={label ?? state}
          issueCount={issues.length}
          onInfoClick={onInfoClick}
        />
      )}

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
            defaultBackend={defaultBackend}
            onProfileChange={onProfileChange}
            onDispatch={onDispatch}
          />
        ))}
      </div>
    </div>
  );
}
