import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { LoadingSpinner } from './LoadingSpinner';

describe('LoadingSpinner', () => {
  it('renders optional label', () => {
    render(<LoadingSpinner text="Loading experiments" />);
    expect(screen.getByText('Loading experiments')).toBeInTheDocument();
  });
});
