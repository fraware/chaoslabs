import React, { useMemo, useCallback } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import { clsx } from 'clsx';

export interface Column<T> {
  id: string;
  header: string;
  accessor: keyof T | ((item: T) => any);
  width?: number;
  minWidth?: number;
  maxWidth?: number;
  sortable?: boolean;
  Cell?: React.ComponentType<{ value: any; row: T; index: number }>;
}

interface Props<T> {
  data: T[];
  columns: Column<T>[];
  rowHeight?: number;
  overscan?: number;
  className?: string;
  onRowClick?: (row: T, index: number) => void;
  sortBy?: string;
  sortDirection?: 'asc' | 'desc';
  onSort?: (columnId: string, direction: 'asc' | 'desc') => void;
  isLoading?: boolean;
  estimatedSize?: number;
}

export function VirtualizedTable<T extends Record<string, any>>({
  data,
  columns,
  rowHeight = 48,
  overscan = 5,
  className,
  onRowClick,
  sortBy,
  sortDirection,
  onSort,
  isLoading = false,
  estimatedSize = 50000,
}: Props<T>) {
  const parentRef = React.useRef<HTMLDivElement>(null);

  const rowVirtualizer = useVirtualizer({
    count: data.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => rowHeight,
    overscan,
  });

  const handleSort = useCallback((columnId: string) => {
    if (!onSort) return;
    
    const newDirection = sortBy === columnId && sortDirection === 'asc' ? 'desc' : 'asc';
    onSort(columnId, newDirection);
  }, [sortBy, sortDirection, onSort]);

  const getCellValue = useCallback((row: T, column: Column<T>) => {
    if (typeof column.accessor === 'function') {
      return column.accessor(row);
    }
    return row[column.accessor];
  }, []);

  const items = rowVirtualizer.getVirtualItems();

  return (
    <div className={clsx('h-full flex flex-col', className)}>
      {/* Header */}
      <div className="flex bg-gray-50 border-b border-gray-200 sticky top-0 z-10">
        {columns.map((column) => (
          <div
            key={column.id}
            className={clsx(
              'flex items-center px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider',
              column.sortable && 'cursor-pointer hover:bg-gray-100',
              sortBy === column.id && 'bg-chaos-50 text-chaos-700'
            )}
            style={{
              width: column.width || 'auto',
              minWidth: column.minWidth || 100,
              maxWidth: column.maxWidth || 'none',
              flex: column.width ? 'none' : 1,
            }}
            onClick={() => column.sortable && handleSort(column.id)}
          >
            <span>{column.header}</span>
            {column.sortable && sortBy === column.id && (
              <svg
                className={clsx(
                  'ml-2 h-4 w-4 transform',
                  sortDirection === 'desc' && 'rotate-180'
                )}
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M5 15l7-7 7 7"
                />
              </svg>
            )}
          </div>
        ))}
      </div>

      {/* Table body */}
      <div
        ref={parentRef}
        className="flex-1 overflow-auto"
        style={{
          height: `400px`,
        }}
      >
        {isLoading ? (
          <div className="flex items-center justify-center h-32">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-chaos-600"></div>
          </div>
        ) : data.length === 0 ? (
          <div className="flex items-center justify-center h-32 text-gray-500">
            No data available
          </div>
        ) : (
          <div
            style={{
              height: `${rowVirtualizer.getTotalSize()}px`,
              width: '100%',
              position: 'relative',
            }}
          >
            {items.map((virtualItem) => {
              const row = data[virtualItem.index];
              
              return (
                <div
                  key={virtualItem.key}
                  className={clsx(
                    'absolute top-0 left-0 w-full flex border-b border-gray-200',
                    onRowClick && 'cursor-pointer hover:bg-gray-50',
                    virtualItem.index % 2 === 0 ? 'bg-white' : 'bg-gray-50/50'
                  )}
                  style={{
                    height: `${virtualItem.size}px`,
                    transform: `translateY(${virtualItem.start}px)`,
                  }}
                  onClick={() => onRowClick?.(row, virtualItem.index)}
                >
                  {columns.map((column) => {
                    const value = getCellValue(row, column);
                    
                    return (
                      <div
                        key={column.id}
                        className="flex items-center px-4 py-3 text-sm text-gray-900"
                        style={{
                          width: column.width || 'auto',
                          minWidth: column.minWidth || 100,
                          maxWidth: column.maxWidth || 'none',
                          flex: column.width ? 'none' : 1,
                        }}
                      >
                        {column.Cell ? (
                          <column.Cell 
                            value={value} 
                            row={row} 
                            index={virtualItem.index} 
                          />
                        ) : (
                          <span className="truncate">{value}</span>
                        )}
                      </div>
                    );
                  })}
                </div>
              );
            })}
          </div>
        )}
      </div>

      {/* Footer with performance info */}
      {data.length > 0 && (
        <div className="flex items-center justify-between px-4 py-2 text-xs text-gray-500 border-t border-gray-200">
          <span>
            Showing {items.length} of {data.length} rows (virtualized)
          </span>
          <span>
            Estimated size: {Math.round(rowVirtualizer.getTotalSize())}px
          </span>
        </div>
      )}
    </div>
  );
}

// Custom cell components
export const StatusCell: React.FC<{ value: string }> = ({ value }) => {
  const getStatusColor = (status: string) => {
    switch (status.toLowerCase()) {
      case 'running':
        return 'bg-green-100 text-green-800';
      case 'completed':
        return 'bg-blue-100 text-blue-800';
      case 'failed':
        return 'bg-red-100 text-red-800';
      case 'pending':
        return 'bg-yellow-100 text-yellow-800';
      default:
        return 'bg-gray-100 text-gray-800';
    }
  };

  return (
    <span className={clsx(
      'inline-flex px-2 py-1 text-xs font-semibold rounded-full',
      getStatusColor(value)
    )}>
      {value}
    </span>
  );
};

export const DateCell: React.FC<{ value: string | Date }> = ({ value }) => {
  const date = typeof value === 'string' ? new Date(value) : value;
  
  return (
    <span className="text-gray-900">
      {date.toLocaleDateString()} {date.toLocaleTimeString()}
    </span>
  );
};

export const DurationCell: React.FC<{ value: number }> = ({ value }) => {
  const formatDuration = (seconds: number) => {
    if (seconds < 60) return `${seconds}s`;
    if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
    return `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`;
  };

  return <span className="text-gray-900">{formatDuration(value)}</span>;
};