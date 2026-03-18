import { useDraggable, useDroppable } from '@dnd-kit/core';
import { CSS } from '@dnd-kit/utilities';
import IssueCard from './IssueCard';
import type { TrackerIssue } from '../../types/symphony';

function DraggableCard({
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
    ? {
        transform: CSS.Translate.toString(transform),
        zIndex: isDragging ? 999 : undefined,
      }
    : undefined;

  return (
    <div ref={setNodeRef} style={style} {...attributes} {...listeners}>
      <IssueCard issue={issue} isDragging={isDragging} onSelect={onSelect} />
    </div>
  );
}

interface ColumnProps {
  state: string;
  issues: TrackerIssue[];
  isOver: boolean;
  onSelect: (id: string) => void;
}

export default function BoardColumn({ state, issues, isOver, onSelect }: ColumnProps) {
  const { setNodeRef } = useDroppable({ id: state });

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
        <span className="truncate text-xs font-semibold tracking-wide text-gray-600 uppercase dark:text-gray-400">
          {state}
        </span>
        <span className="ml-2 flex h-5 min-w-[20px] flex-shrink-0 items-center justify-center rounded-full border border-gray-200 bg-white px-1.5 text-[10px] font-bold text-gray-500 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-400">
          {issues.length}
        </span>
      </div>
      <div className="max-h-[calc(100vh-300px)] min-h-[80px] flex-1 space-y-1.5 overflow-y-auto px-2 pb-2">
        {issues.map((issue) => (
          <DraggableCard key={issue.identifier} issue={issue} onSelect={onSelect} />
        ))}
      </div>
    </div>
  );
}
