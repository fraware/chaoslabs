import React from 'react';
import { clsx } from 'clsx';

interface Props {
  size?: 'sm' | 'md' | 'lg';
  className?: string;
  text?: string;
}

export function LoadingSpinner({ size = 'md', className, text }: Props) {
  const sizeClasses = {
    sm: 'h-4 w-4',
    md: 'h-8 w-8',
    lg: 'h-12 w-12',
  };

  return (
    <div className={clsx('flex flex-col items-center justify-center p-8', className)}>
      <div
        className={clsx(
          'animate-spin rounded-full border-2 border-chaos-200 border-t-chaos-600',
          sizeClasses[size]
        )}
      />
      {text && (
        <p className="mt-4 text-sm text-gray-600">{text}</p>
      )}
    </div>
  );
}
