import React from 'react';
import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import {
  BeakerIcon,
  ClockIcon,
  CheckCircleIcon,
  ExclamationTriangleIcon,
  ArrowTrendingUpIcon,
} from '@heroicons/react/24/outline';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, BarChart, Bar } from 'recharts';
import { useExperimentUpdates } from '../hooks/useWebSocket';
import { LoadingSpinner } from '../components/LoadingSpinner';

interface DashboardStats {
  total_experiments: number;
  running_experiments: number;
  completed_experiments: number;
  failed_experiments: number;
  avg_duration: number;
  success_rate: number;
}

interface ExperimentTrend {
  date: string;
  experiments: number;
  success_rate: number;
}

interface TypeDistribution {
  type: string;
  count: number;
  success_rate: number;
}

// Mock data
const mockStats: DashboardStats = {
  total_experiments: 1247,
  running_experiments: 23,
  completed_experiments: 1156,
  failed_experiments: 68,
  avg_duration: 342,
  success_rate: 94.5,
};

const mockTrends: ExperimentTrend[] = Array.from({ length: 30 }, (_, i) => ({
  date: new Date(Date.now() - (29 - i) * 24 * 60 * 60 * 1000).toISOString().split('T')[0],
  experiments: Math.floor(Math.random() * 50) + 20,
  success_rate: Math.random() * 20 + 80,
}));

const mockTypeDistribution: TypeDistribution[] = [
  { type: 'Network Latency', count: 456, success_rate: 96.2 },
  { type: 'CPU Stress', count: 312, success_rate: 93.8 },
  { type: 'Memory Stress', count: 234, success_rate: 91.5 },
  { type: 'Network Loss', count: 156, success_rate: 95.1 },
  { type: 'Process Kill', count: 89, success_rate: 87.6 },
];

async function fetchDashboardStats(): Promise<DashboardStats> {
  await new Promise(resolve => setTimeout(resolve, 500));
  return mockStats;
}

async function fetchExperimentTrends(): Promise<ExperimentTrend[]> {
  await new Promise(resolve => setTimeout(resolve, 300));
  return mockTrends;
}

async function fetchTypeDistribution(): Promise<TypeDistribution[]> {
  await new Promise(resolve => setTimeout(resolve, 200));
  return mockTypeDistribution;
}

const StatCard: React.FC<{
  title: string;
  value: string | number;
  icon: React.ComponentType<{ className?: string }>;
  color: string;
  trend?: string;
}> = ({ title, value, icon: Icon, color, trend }) => (
  <div className="bg-white overflow-hidden shadow rounded-lg">
    <div className="p-5">
      <div className="flex items-center">
        <div className="flex-shrink-0">
          <Icon className={`h-6 w-6 ${color}`} />
        </div>
        <div className="ml-5 w-0 flex-1">
          <dl>
            <dt className="text-sm font-medium text-gray-500 truncate">{title}</dt>
            <dd className="flex items-baseline">
              <div className="text-2xl font-semibold text-gray-900">{value}</div>
              {trend && (
                <div className="ml-2 flex items-baseline text-sm font-semibold text-green-600">
                  <ArrowTrendingUpIcon className="h-4 w-4 mr-1" />
                  {trend}
                </div>
              )}
            </dd>
          </dl>
        </div>
      </div>
    </div>
  </div>
);

export default function Dashboard() {
  // Connect to real-time updates
  useExperimentUpdates();

  const { data: stats, isLoading: isLoadingStats } = useQuery({
    queryKey: ['dashboard-stats'],
    queryFn: fetchDashboardStats,
    staleTime: 30 * 1000,
    refetchInterval: 60 * 1000,
  });

  const { data: trends, isLoading: isLoadingTrends } = useQuery({
    queryKey: ['experiment-trends'],
    queryFn: fetchExperimentTrends,
    staleTime: 5 * 60 * 1000,
  });

  const { data: typeDistribution, isLoading: isLoadingTypes } = useQuery({
    queryKey: ['type-distribution'],
    queryFn: fetchTypeDistribution,
    staleTime: 5 * 60 * 1000,
  });

  if (isLoadingStats) {
    return <LoadingSpinner text="Loading dashboard..." />;
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="md:flex md:items-center md:justify-between">
        <div className="flex-1 min-w-0">
          <h1 className="text-2xl font-bold text-gray-900">Dashboard</h1>
          <p className="text-gray-600">
            Overview of your chaos engineering experiments and system health
          </p>
        </div>
        <div className="mt-4 flex md:mt-0 md:ml-4">
          <Link
            to="/experiments"
            className="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-chaos-600 hover:bg-chaos-700"
          >
            View All Experiments
          </Link>
        </div>
      </div>

      {/* Stats Cards */}
      {stats && (
        <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
          <StatCard
            title="Total Experiments"
            value={stats.total_experiments.toLocaleString()}
            icon={BeakerIcon}
            color="text-blue-600"
            trend="+12%"
          />
          <StatCard
            title="Currently Running"
            value={stats.running_experiments}
            icon={ClockIcon}
            color="text-yellow-600"
          />
          <StatCard
            title="Success Rate"
            value={`${stats.success_rate}%`}
            icon={CheckCircleIcon}
            color="text-green-600"
            trend="+2.1%"
          />
          <StatCard
            title="Avg Duration"
            value={`${Math.floor(stats.avg_duration / 60)}m ${stats.avg_duration % 60}s`}
            icon={ArrowTrendingUpIcon}
            color="text-purple-600"
            trend="-5%"
          />
        </div>
      )}

      {/* Charts Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Experiment Trends */}
        <div className="bg-white p-6 rounded-lg shadow">
          <h2 className="text-lg font-medium text-gray-900 mb-4">
            Experiment Trends (30 days)
          </h2>
          {isLoadingTrends ? (
            <LoadingSpinner size="md" />
          ) : (
            <ResponsiveContainer width="100%" height={300}>
              <LineChart data={trends}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis
                  dataKey="date"
                  tickFormatter={(value) => new Date(value).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })}
                />
                <YAxis />
                <Tooltip
                  labelFormatter={(value) => new Date(value).toLocaleDateString()}
                />
                <Line
                  type="monotone"
                  dataKey="experiments"
                  stroke="#0ea5e9"
                  strokeWidth={2}
                  dot={{ r: 4 }}
                />
              </LineChart>
            </ResponsiveContainer>
          )}
        </div>

        {/* Success Rate Trends */}
        <div className="bg-white p-6 rounded-lg shadow">
          <h2 className="text-lg font-medium text-gray-900 mb-4">
            Success Rate Trends
          </h2>
          {isLoadingTrends ? (
            <LoadingSpinner size="md" />
          ) : (
            <ResponsiveContainer width="100%" height={300}>
              <LineChart data={trends}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis
                  dataKey="date"
                  tickFormatter={(value) => new Date(value).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })}
                />
                <YAxis domain={[80, 100]} />
                <Tooltip
                  labelFormatter={(value) => new Date(value).toLocaleDateString()}
                  formatter={(value: number) => [`${value.toFixed(1)}%`, 'Success Rate']}
                />
                <Line
                  type="monotone"
                  dataKey="success_rate"
                  stroke="#10b981"
                  strokeWidth={2}
                  dot={{ r: 4 }}
                />
              </LineChart>
            </ResponsiveContainer>
          )}
        </div>
      </div>

      {/* Experiment Type Distribution */}
      <div className="bg-white p-6 rounded-lg shadow">
        <h2 className="text-lg font-medium text-gray-900 mb-4">
          Experiment Type Distribution
        </h2>
        {isLoadingTypes ? (
          <LoadingSpinner size="md" />
        ) : (
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            <ResponsiveContainer width="100%" height={300}>
              <BarChart data={typeDistribution}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis
                  dataKey="type"
                  angle={-45}
                  textAnchor="end"
                  height={80}
                  fontSize={12}
                />
                <YAxis />
                <Tooltip />
                <Bar dataKey="count" fill="#0ea5e9" />
              </BarChart>
            </ResponsiveContainer>

            {/* Type Stats Table */}
            <div className="space-y-3">
              {typeDistribution?.map((type, index) => (
                <div key={index} className="flex items-center justify-between p-3 bg-gray-50 rounded-lg">
                  <div>
                    <div className="font-medium text-gray-900">{type.type}</div>
                    <div className="text-sm text-gray-600">{type.count} experiments</div>
                  </div>
                  <div className="text-right">
                    <div className="font-medium text-gray-900">{type.success_rate}%</div>
                    <div className="text-sm text-gray-600">success rate</div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>

      {/* Recent Activity */}
      <div className="bg-white shadow rounded-lg">
        <div className="px-6 py-4 border-b border-gray-200">
          <h2 className="text-lg font-medium text-gray-900">Recent Activity</h2>
        </div>
        <div className="p-6">
          <div className="space-y-4">
            {[
              { type: 'success', message: 'Network latency experiment completed successfully', time: '2 minutes ago' },
              { type: 'warning', message: 'Memory stress test showing high resource usage', time: '15 minutes ago' },
              { type: 'info', message: 'New experiment scheduled for tomorrow', time: '1 hour ago' },
              { type: 'error', message: 'CPU stress experiment failed due to timeout', time: '2 hours ago' },
            ].map((activity, index) => (
              <div key={index} className="flex items-center space-x-3">
                <div className={`flex-shrink-0 w-2 h-2 rounded-full ${
                  activity.type === 'success' ? 'bg-green-400' :
                  activity.type === 'warning' ? 'bg-yellow-400' :
                  activity.type === 'error' ? 'bg-red-400' : 'bg-blue-400'
                }`} />
                <div className="flex-1 min-w-0">
                  <p className="text-sm text-gray-900">{activity.message}</p>
                  <p className="text-xs text-gray-500">{activity.time}</p>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}