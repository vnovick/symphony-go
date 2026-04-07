/**
 * Deterministic accent colors for agent profiles.
 *
 * Hashes the profile name into a curated palette so each agent gets a
 * consistent, visually distinct color across sessions and users.
 */

// Curated palette — high-contrast on dark backgrounds, visually distinct pairs.
const PALETTE = [
  { hue: 215, accent: '#58a6ff', bg: 'rgba(88,166,255,0.12)' }, // blue
  { hue: 270, accent: '#bc8cff', bg: 'rgba(188,140,255,0.15)' }, // purple
  { hue: 25, accent: '#ffa657', bg: 'rgba(255,166,87,0.15)' }, // orange
  { hue: 330, accent: '#f778ba', bg: 'rgba(247,120,186,0.15)' }, // pink
  { hue: 155, accent: '#3fb950', bg: 'rgba(63,185,80,0.12)' }, // green
  { hue: 180, accent: '#39d3c5', bg: 'rgba(57,211,197,0.12)' }, // teal
  { hue: 45, accent: '#d29922', bg: 'rgba(210,153,34,0.15)' }, // gold
  { hue: 0, accent: '#f85149', bg: 'rgba(248,81,73,0.12)' }, // red
  { hue: 195, accent: '#56d4dd', bg: 'rgba(86,212,221,0.12)' }, // cyan
  { hue: 290, accent: '#d2a8ff', bg: 'rgba(210,168,255,0.15)' }, // lavender
] as const;

// Muted fallback for the "Unassigned" column.
export const UNASSIGNED_COLOR = {
  accent: '#484f58',
  bg: 'rgba(72,79,88,0.12)',
  gradient: 'linear-gradient(135deg, #30363d, #484f58)',
} as const;

function hashString(str: string): number {
  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    hash = ((hash << 5) - hash + str.charCodeAt(i)) | 0;
  }
  return Math.abs(hash);
}

export interface ProfileColor {
  accent: string; // e.g. '#58a6ff' — used for borders, badges, top-edge
  bg: string; // e.g. 'rgba(88,166,255,0.12)' — tinted backgrounds
  gradient: string; // e.g. 'linear-gradient(...)' — avatar background
}

export function profileColor(name: string): ProfileColor {
  const entry = PALETTE[hashString(name) % PALETTE.length];
  return {
    accent: entry.accent,
    bg: entry.bg,
    gradient: `linear-gradient(135deg, ${entry.accent}cc, ${entry.accent})`,
  };
}

/** Two-letter monogram for the avatar. */
export function profileInitials(name: string): string {
  const words = name
    .trim()
    .split(/[\s_-]+/)
    .filter(Boolean);
  if (words.length >= 2) {
    return (words[0][0] + words[1][0]).toUpperCase();
  }
  return name.slice(0, 2).toUpperCase();
}
