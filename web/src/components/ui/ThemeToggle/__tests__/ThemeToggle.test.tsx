import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { ThemeToggle } from '../ThemeToggle';
import { ThemeProvider } from '../../../../context/ThemeContext';

function renderToggle() {
  return render(
    <ThemeProvider>
      <ThemeToggle />
    </ThemeProvider>,
  );
}

describe('ThemeToggle', () => {
  beforeEach(() => {
    document.documentElement.removeAttribute('data-theme');
    document.documentElement.classList.remove('dark');
    localStorage.clear();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders a button', () => {
    renderToggle();
    expect(screen.getByRole('button')).toBeInTheDocument();
  });

  it('has an accessible label', () => {
    renderToggle();
    const btn = screen.getByRole('button');
    expect(btn).toHaveAttribute('aria-label');
  });

  it('sets data-theme=light on <html> when toggled from dark', () => {
    // Provider defaults to dark when no localStorage value
    renderToggle();
    fireEvent.click(screen.getByRole('button'));
    expect(document.documentElement.getAttribute('data-theme')).toBe('light');
  });

  it('sets data-theme=dark on <html> when toggled from light', () => {
    localStorage.setItem('theme', 'light'); // Provider reads this on init
    renderToggle();
    fireEvent.click(screen.getByRole('button'));
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark');
  });

  it('persists the chosen theme to localStorage', () => {
    renderToggle(); // starts dark
    fireEvent.click(screen.getByRole('button')); // → light
    expect(localStorage.getItem('theme')).toBe('light');
  });

  it('reads initial theme from localStorage', () => {
    localStorage.setItem('theme', 'light');
    renderToggle();
    expect(document.documentElement.getAttribute('data-theme')).toBe('light');
  });

  it('falls back to dark when localStorage has no theme', () => {
    renderToggle();
    expect(document.documentElement.getAttribute('data-theme')).not.toBe('light');
  });
});
