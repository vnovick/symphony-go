import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { Card } from '../Card';

describe('Card', () => {
  it('renders children', () => {
    render(<Card>hello</Card>);
    expect(screen.getByText('hello')).toBeInTheDocument();
  });

  it('applies default variant class', () => {
    const { container } = render(<Card>x</Card>);
    expect(container.firstChild).toBeInTheDocument();
  });

  it('applies elevated variant', () => {
    const { container } = render(<Card variant="elevated">x</Card>);
    expect(container.firstChild).toHaveAttribute('data-variant', 'elevated');
  });

  it('applies outline variant', () => {
    const { container } = render(<Card variant="outline">x</Card>);
    expect(container.firstChild).toHaveAttribute('data-variant', 'outline');
  });

  it('applies padding size none', () => {
    const { container } = render(<Card padding="none">x</Card>);
    expect(container.firstChild).toHaveAttribute('data-padding', 'none');
  });

  it('applies padding size lg', () => {
    const { container } = render(<Card padding="lg">x</Card>);
    expect(container.firstChild).toHaveAttribute('data-padding', 'lg');
  });

  describe('Card.Header', () => {
    it('renders header children', () => {
      render(
        <Card>
          <Card.Header>Title</Card.Header>
        </Card>,
      );
      expect(screen.getByText('Title')).toBeInTheDocument();
    });
  });

  describe('Card.Body', () => {
    it('renders body children', () => {
      render(
        <Card>
          <Card.Body>Body content</Card.Body>
        </Card>,
      );
      expect(screen.getByText('Body content')).toBeInTheDocument();
    });
  });

  describe('Card.Footer', () => {
    it('renders footer children', () => {
      render(
        <Card>
          <Card.Footer>Footer</Card.Footer>
        </Card>,
      );
      expect(screen.getByText('Footer')).toBeInTheDocument();
    });
  });
});
