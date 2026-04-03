import { useState, useEffect } from 'react';

interface ReviewerCardProps {
  reviewerProfile: string;
  autoReview: boolean;
  availableProfiles: string[];
  onSave: (profile: string, autoReview: boolean) => Promise<boolean>;
}

export function ReviewerCard({
  reviewerProfile,
  autoReview,
  availableProfiles,
  onSave,
}: ReviewerCardProps) {
  const [profile, setProfile] = useState(reviewerProfile);
  const [auto, setAuto] = useState(autoReview);
  const [saving, setSaving] = useState(false);

  // Sync local state when props change (e.g. after refreshSnapshot).
  useEffect(() => { setProfile(reviewerProfile); }, [reviewerProfile]);
  useEffect(() => { setAuto(autoReview); }, [autoReview]);

  const dirty = profile !== reviewerProfile || auto !== autoReview;

  const handleSave = async () => {
    setSaving(true);
    await onSave(profile, auto);
    setSaving(false);
  };

  return (
    <div className="space-y-4">
      <div>
        <label className="block text-xs font-medium text-theme-text-secondary mb-1">
          Reviewer profile
        </label>
        <select
          value={profile}
          onChange={(e) => { setProfile(e.target.value); }}
          className="w-full rounded-[var(--radius-sm)] border px-3 py-2 text-[13px] cursor-pointer focus:outline-none bg-[var(--panel-strong)] border-[var(--line)] text-[var(--text)]"
        >
          <option value="">None (disabled)</option>
          {availableProfiles.map((p) => (
            <option key={p} value={p}>{p}</option>
          ))}
        </select>
        <p className="mt-1 text-[10px] text-theme-muted">
          Select a profile to use for AI code review. The reviewer runs as a regular worker with the
          profile's command, backend, and prompt.
        </p>
      </div>

      <label className="flex items-center gap-2 cursor-pointer">
        <input
          type="checkbox"
          checked={auto}
          onChange={(e) => { setAuto(e.target.checked); }}
          disabled={!profile}
          className="rounded border-theme-line"
        />
        <span className={`text-sm ${!profile ? 'text-theme-muted' : 'text-theme-text'}`}>
          Auto-review after agent succeeds
        </span>
      </label>
      {auto && profile && (
        <p className="text-[10px] text-theme-muted pl-6">
          A reviewer worker will be automatically dispatched using the <strong>{profile}</strong> profile
          after each successful agent run.
        </p>
      )}

      {dirty && (
        <button
          onClick={() => { void handleSave(); }}
          disabled={saving}
          className="rounded-[var(--radius-sm)] px-4 py-2 text-sm font-medium text-white transition-colors disabled:opacity-50 bg-theme-accent"
        >
          {saving ? 'Saving…' : 'Save'}
        </button>
      )}
    </div>
  );
}
