import React, { useState, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { PlusIcon, FunnelIcon } from '@heroicons/react/24/outline';
import { VirtualizedTable, Column, StatusCell, DateCell, DurationCell } from '../components/VirtualizedTable';
import { LoadingSpinner } from '../components/LoadingSpinner';
import { useExperimentUpdates } from '../hooks/useWebSocket';
import { clsx } from 'clsx';

interface Experiment {
  id: string;
  name: string;
  description: string;
  experiment_type: string;
  target: string;
  status: 'pending' | 'running' | 'completed' | 'failed';
  duration: number;
  created_at: string;
  updated_at: string;
  start_time?: string;
  end_time?: string;
  agent_count: number;
}

const FILTER_OPTIONS = [
  { value: 'all', label: 'All Experiments' },
  { value: 'running', label: 'Running' },
  { value: 'completed', label: 'Completed' },
  { value: 'failed', label: 'Failed' },
  { value: 'pending', label: 'Pending' },
];

const SORT_OPTIONS = [
  { value: 'created_at', label: 'Created Date' },
  { value: 'name', label: 'Name' },
  { value: 'status', label: 'Status' },
  { value: 'duration', label: 'Duration' },
];

// Mock data generator for large dataset demo
function generateMockExperiments(count: number): Experiment[] {
  const types = ['network_latency', 'network_loss', 'cpu_stress', 'memory_stress', 'process_kill'];
  const statuses: Experiment['status'][] = ['pending', 'running', 'completed', 'failed'];
  const targets = ['web-server', 'database', 'cache', 'api-gateway', 'load-balancer'];
  
  return Array.from({ length: count }, (_, i) => ({
    id: `exp-${i + 1}`,
    name: `Experiment ${i + 1}`,
    description: `Test experiment for ${types[i % types.length]} on ${targets[i % targets.length]}`,
    experiment_type: types[i % types.length],
    target: targets[i % targets.length],
    status: statuses[i % statuses.length],
    duration: Math.floor(Math.random() * 3600) + 60,
    created_at: new Date(Date.now() - Math.random() * 30 * 24 * 60 * 60 * 1000).toISOString(),
    updated_at: new Date(Date.now() - Math.random() * 24 * 60 * 60 * 1000).toISOString(),
    start_time: Math.random() > 0.5 ? new Date(Date.now() - Math.random() * 24 * 60 * 60 * 1000).toISOString() : undefined,
    end_time: Math.random() > 0.7 ? new Date(Date.now() - Math.random() * 12 * 60 * 60 * 1000).toISOString() : undefined,
    agent_count: Math.floor(Math.random() * 5) + 1,
  }));
}

async function fetchExperiments(): Promise<Experiment[]> {
  // In a real implementation, this would fetch from the API
  // For demo purposes, return mock data including a large dataset
  const response = await fetch('/api/experiments');
  
  if (!response.ok) {
    // Fallback to mock data if API is not available
    return generateMockExperiments(50000); // Generate 50k records for performance demo
  }
  
  const data = await response.json();
  return data.experiments || [];
}

export default function ExperimentsList() {
  const [statusFilter, setStatusFilter] = useState('all');
  const [searchTerm, setSearchTerm] = useState('');
  const [sortBy, setSortBy] = useState('created_at');
  const [sortDirection, setSortDirection] = useState<'asc' | 'desc'>('desc');

  // Connect to real-time updates
  useExperimentUpdates();

  const { data: experiments = [], isLoading, error } = useQuery({
    queryKey: ['experiments'],
    queryFn: fetchExperiments,
    staleTime: 30 * 1000, // 30 seconds
    refetchInterval: 60 * 1000, // Refetch every minute
  });

  // Filter and sort experiments
  const filteredAndSortedExperiments = useMemo(() => {
    let filtered = experiments;

    // Apply status filter
    if (statusFilter !== 'all') {
      filtered = filtered.filter(exp => exp.status === statusFilter);
    }

    // Apply search filter
    if (searchTerm) {
      const searchLower = searchTerm.toLowerCase();
      filtered = filtered.filter(exp =>
        exp.name.toLowerCase().includes(searchLower) ||
        exp.description.toLowerCase().includes(searchLower) ||
        exp.target.toLowerCase().includes(searchLower) ||
        exp.experiment_type.toLowerCase().includes(searchLower)
      );
    }

    // Sort
    filtered.sort((a, b) => {
      let aVal: any = a[sortBy as keyof Experiment];
      let bVal: any = b[sortBy as keyof Experiment];

      if (sortBy === 'created_at' || sortBy === 'updated_at') {
        aVal = new Date(aVal).getTime();
        bVal = new Date(bVal).getTime();
      }

      if (typeof aVal === 'string' && typeof bVal === 'string') {
        aVal = aVal.toLowerCase();
        bVal = bVal.toLowerCase();
      }

      if (aVal < bVal) return sortDirection === 'asc' ? -1 : 1;
      if (aVal > bVal) return sortDirection === 'asc' ? 1 : -1;
      return 0;
    });

    return filtered;
  }, [experiments, statusFilter, searchTerm, sortBy, sortDirection]);

  const columns: Column<Experiment>[] = [
    {
      id: 'name',
      header: 'Name',
      accessor: 'name',
      sortable: true,
      width: 200,
      Cell: ({ value, row }) => (
        <Link
          to={`/experiments/${row.id}`}
          className="font-medium text-chaos-600 hover:text-chaos-800"
        >
          {value}
        </Link>
      ),
    },
    {
      id: 'status',
      header: 'Status',
      accessor: 'status',
      sortable: true,
      width: 120,
      Cell: ({ value }) => <StatusCell value={value} />,
    },
    {
      id: 'experiment_type',
      header: 'Type',
      accessor: 'experiment_type',
      sortable: true,
      width: 150,
    },
    {
      id: 'target',
      header: 'Target',
      accessor: 'target',
      sortable: true,
      width: 150,
    },
    {
      id: 'duration',
      header: 'Duration',
      accessor: 'duration',
      sortable: true,
      width: 100,
      Cell: ({ value }) => <DurationCell value={value} />,
    },
    {
      id: 'agent_count',
      header: 'Agents',
      accessor: 'agent_count',
      sortable: true,
      width: 80,
    },
    {
      id: 'created_at',
      header: 'Created',
      accessor: 'created_at',
      sortable: true,
      width: 160,
      Cell: ({ value }) => <DateCell value={value} />,
    },
  ];

  const handleSort = (columnId: string, direction: 'asc' | 'desc') => {
    setSortBy(columnId);
    setSortDirection(direction);
  };

  if (error) {
    return (
      <div className="text-center py-12">
        <div className="text-red-600 mb-4">Failed to load experiments</div>
        <button
          onClick={() => window.location.reload()}
          className="px-4 py-2 bg-chaos-600 text-white rounded-md hover:bg-chaos-700"
        >
          Retry
        </button>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Experiments</h1>
          <p className="text-gray-600">
            Manage and monitor your chaos engineering experiments
          </p>
        </div>
        
        <Link
          to="/experiments/new"
          className="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-chaos-600 hover:bg-chaos-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-chaos-500"
        >
          <PlusIcon className="-ml-1 mr-2 h-5 w-5" />
          New Experiment
        </Link>
      </div>

      {/* Filters and Search */}
      <div className="bg-white shadow rounded-lg p-6">
        <div className="flex flex-col sm:flex-row gap-4">
          {/* Search */}
          <div className="flex-1">
            <input
              type="text"
              placeholder="Search experiments..."
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              className="block w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm placeholder-gray-400 focus:outline-none focus:ring-chaos-500 focus:border-chaos-500"
            />
          </div>

          {/* Status Filter */}
          <div className="sm:w-48">
            <select
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value)}
              className="block w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-chaos-500 focus:border-chaos-500"
            >
              {FILTER_OPTIONS.map(option => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          </div>

          {/* Sort */}
          <div className="sm:w-48">
            <select
              value={sortBy}
              onChange={(e) => setSortBy(e.target.value)}
              className="block w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-chaos-500 focus:border-chaos-500"
            >
              {SORT_OPTIONS.map(option => (
                <option key={option.value} value={option.value}>
                  Sort by {option.label}
                </option>
              ))}
            </select>
          </div>
        </div>

        {/* Results summary */}
        <div className="mt-4 text-sm text-gray-600">
          Showing {filteredAndSortedExperiments.length} of {experiments.length} experiments
          {experiments.length >= 10000 && (
            <span className="ml-2 px-2 py-1 bg-blue-100 text-blue-800 rounded-full text-xs">
              Large dataset - virtualized for performance
            </span>
          )}
        </div>
      </div>

      {/* Experiments Table */}
      <div className="bg-white shadow rounded-lg">
        <div className="h-[600px]">
          <VirtualizedTable
            data={filteredAndSortedExperiments}
            columns={columns}
            isLoading={isLoading}
            sortBy={sortBy}
            sortDirection={sortDirection}
            onSort={handleSort}
            rowHeight={56}
            overscan={10}
          />
        </div>
      </div>
    </div>
  );
}
