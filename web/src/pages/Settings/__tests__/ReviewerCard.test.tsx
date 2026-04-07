import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { ReviewerCard } from '../ReviewerCard';

describe('ReviewerCard', () => {
  const profiles = ['reviewer', 'auditor', 'linter'];
  let onSave: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    onSave = vi.fn().mockResolvedValue(true);
  });

  it('renders with no profile selected', () => {
    render(
      <ReviewerCard
        reviewerProfile=""
        autoReview={false}
        availableProfiles={profiles}
        onSave={onSave}
      />,
    );
    const select = screen.getByRole('combobox');
    expect(select).toHaveValue('');
    expect(screen.getByText('None (disabled)')).toBeInTheDocument();
  });

  it('renders with a profile selected', () => {
    render(
      <ReviewerCard
        reviewerProfile="reviewer"
        autoReview={false}
        availableProfiles={profiles}
        onSave={onSave}
      />,
    );
    const select = screen.getByRole('combobox');
    expect(select).toHaveValue('reviewer');
  });

  it('changing profile marks form dirty and shows Save button', () => {
    render(
      <ReviewerCard
        reviewerProfile=""
        autoReview={false}
        availableProfiles={profiles}
        onSave={onSave}
      />,
    );
    expect(screen.queryByRole('button', { name: /save/i })).toBeNull();

    fireEvent.change(screen.getByRole('combobox'), { target: { value: 'auditor' } });

    expect(screen.getByRole('button', { name: /save/i })).toBeInTheDocument();
  });

  it('toggling auto-review marks form dirty', () => {
    render(
      <ReviewerCard
        reviewerProfile="reviewer"
        autoReview={false}
        availableProfiles={profiles}
        onSave={onSave}
      />,
    );
    expect(screen.queryByRole('button', { name: /save/i })).toBeNull();

    fireEvent.click(screen.getByRole('checkbox'));

    expect(screen.getByRole('button', { name: /save/i })).toBeInTheDocument();
  });

  it('disables auto-review checkbox when no profile selected', () => {
    render(
      <ReviewerCard
        reviewerProfile=""
        autoReview={false}
        availableProfiles={profiles}
        onSave={onSave}
      />,
    );
    expect(screen.getByRole('checkbox')).toBeDisabled();
  });

  it('save button calls onSave with current values', async () => {
    render(
      <ReviewerCard
        reviewerProfile=""
        autoReview={false}
        availableProfiles={profiles}
        onSave={onSave}
      />,
    );

    fireEvent.change(screen.getByRole('combobox'), { target: { value: 'reviewer' } });
    fireEvent.click(screen.getByRole('button', { name: /save/i }));

    await waitFor(() => {
      expect(onSave).toHaveBeenCalledWith('reviewer', false);
    });
  });

  it('shows auto-review description when both profile and auto are set', () => {
    render(
      <ReviewerCard
        reviewerProfile="reviewer"
        autoReview={true}
        availableProfiles={profiles}
        onSave={onSave}
      />,
    );
    expect(screen.getByText(/automatically dispatched/)).toBeInTheDocument();
    expect(screen.getByText(/reviewer/, { selector: 'strong' })).toBeInTheDocument();
  });

  it('shows Saving text while save is in progress', async () => {
    let resolveSave: ((v: boolean) => void) | undefined;
    const slowSave = vi.fn(
      () =>
        new Promise<boolean>((resolve) => {
          resolveSave = resolve;
        }),
    );

    render(
      <ReviewerCard
        reviewerProfile=""
        autoReview={false}
        availableProfiles={profiles}
        onSave={slowSave}
      />,
    );

    fireEvent.change(screen.getByRole('combobox'), { target: { value: 'linter' } });
    fireEvent.click(screen.getByRole('button', { name: /save/i }));

    expect(await screen.findByText('Saving…')).toBeInTheDocument();

    if (!resolveSave) throw new Error('resolveSave not set');
    resolveSave(true);

    await waitFor(() => {
      expect(screen.queryByText('Saving…')).toBeNull();
    });
  });
});
