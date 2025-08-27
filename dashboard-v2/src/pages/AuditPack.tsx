import React, { useState, useCallback } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  DocumentArrowDownIcon,
  MagnifyingGlassIcon,
  FolderIcon,
  DocumentTextIcon,
  ClockIcon,
  CheckCircleIcon,
  ExclamationTriangleIcon,
} from '@heroicons/react/24/outline';
import { clsx } from 'clsx';
import { VirtualizedTable, Column, DateCell } from '../components/VirtualizedTable';

interface AuditPack {
  id: string;
  name: string;
  description: string;
  created_at: string;
  size_bytes: number;
  file_count: number;
  status: 'generating' | 'ready' | 'error';
  download_url?: string;
  expires_at: string;
  signature: string;
  merkle_root: string;
  experiments_included: number;
}

interface AuditFile {
  id: string;
  name: string;
  path: string;
  size_bytes: number;
  mime_type: string;
  checksum: string;
  created_at: string;
}

// Mock data for demonstration
const generateMockAuditPacks = (): AuditPack[] => [
  {
    id: 'pack-1',
    name: 'Q4 2023 Chaos Engineering Audit',
    description: 'Complete audit pack for Q4 2023 including all experiments, logs, and metrics',
    created_at: '2023-12-31T23:59:59Z',
    size_bytes: 1024 * 1024 * 150, // 150MB
    file_count: 1250,
    status: 'ready',
    download_url: '/api/audit-packs/pack-1/download',
    expires_at: '2024-12-31T23:59:59Z',
    signature: 'sha256:a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456',
    merkle_root: 'merkle:9876543210abcdef9876543210abcdef9876543210abcdef9876543210abcdef',
    experiments_included: 125,
  },
  {
    id: 'pack-2',
    name: 'Network Latency Experiments - December',
    description: 'All network latency experiments and related data from December 2023',
    created_at: '2023-12-01T00:00:00Z',
    size_bytes: 1024 * 1024 * 75, // 75MB
    file_count: 650,
    status: 'ready',
    download_url: '/api/audit-packs/pack-2/download',
    expires_at: '2024-06-01T00:00:00Z',
    signature: 'sha256:b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef1234567',
    merkle_root: 'merkle:8765432109abcdef8765432109abcdef8765432109abcdef8765432109abcdef',
    experiments_included: 67,
  },
  {
    id: 'pack-3',
    name: 'Security Compliance Audit - November',
    description: 'Security-focused audit pack with all compliance-related experiments',
    created_at: '2023-11-15T12:00:00Z',
    size_bytes: 1024 * 1024 * 200, // 200MB
    file_count: 2100,
    status: 'generating',
    expires_at: '2024-11-15T12:00:00Z',
    signature: '',
    merkle_root: '',
    experiments_included: 89,
  },
];

const generateMockAuditFiles = (packId: string): AuditFile[] => [
  {
    id: 'file-1',
    name: 'experiments.ndjson',
    path: '/data/experiments.ndjson',
    size_bytes: 1024 * 1024 * 25,
    mime_type: 'application/x-ndjson',
    checksum: 'sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
    created_at: '2023-12-31T23:55:00Z',
  },
  {
    id: 'file-2',
    name: 'metrics.parquet',
    path: '/data/metrics.parquet',
    size_bytes: 1024 * 1024 * 45,
    mime_type: 'application/parquet',
    checksum: 'sha256:2345678901abcdef2345678901abcdef2345678901abcdef2345678901abcdef',
    created_at: '2023-12-31T23:56:00Z',
  },
  {
    id: 'file-3',
    name: 'logs.json.gz',
    path: '/logs/aggregated.json.gz',
    size_bytes: 1024 * 1024 * 80,
    mime_type: 'application/gzip',
    checksum: 'sha256:3456789012abcdef3456789012abcdef3456789012abcdef3456789012abcdef',
    created_at: '2023-12-31T23:57:00Z',
  },
];

async function fetchAuditPacks(): Promise<AuditPack[]> {
  // Simulate API call
  await new Promise(resolve => setTimeout(resolve, 500));
  return generateMockAuditPacks();
}

async function fetchAuditFiles(packId: string): Promise<AuditFile[]> {
  // Simulate API call
  await new Promise(resolve => setTimeout(resolve, 300));
  return generateMockAuditFiles(packId);
}

const formatBytes = (bytes: number): string => {
  if (bytes === 0) return '0 Bytes';
  const k = 1024;
  const sizes = ['Bytes', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
};

const StatusIcon: React.FC<{ status: AuditPack['status'] }> = ({ status }) => {
  switch (status) {
    case 'ready':
      return <CheckCircleIcon className="h-5 w-5 text-green-500" />;
    case 'generating':
      return <ClockIcon className="h-5 w-5 text-yellow-500 animate-spin" />;
    case 'error':
      return <ExclamationTriangleIcon className="h-5 w-5 text-red-500" />;
    default:
      return null;
  }
};

export default function AuditPack() {
  const [selectedPack, setSelectedPack] = useState<string | null>(null);
  const [searchTerm, setSearchTerm] = useState('');

  const { data: auditPacks = [], isLoading: isLoadingPacks } = useQuery({
    queryKey: ['audit-packs'],
    queryFn: fetchAuditPacks,
    staleTime: 60 * 1000, // 1 minute
  });

  const { data: auditFiles = [], isLoading: isLoadingFiles } = useQuery({
    queryKey: ['audit-files', selectedPack],
    queryFn: () => selectedPack ? fetchAuditFiles(selectedPack) : [],
    enabled: !!selectedPack,
    staleTime: 60 * 1000,
  });

  const filteredPacks = auditPacks.filter(pack =>
    pack.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
    pack.description.toLowerCase().includes(searchTerm.toLowerCase())
  );

  const auditPackColumns: Column<AuditPack>[] = [
    {
      id: 'status',
      header: 'Status',
      accessor: 'status',
      width: 80,
      Cell: ({ value }) => <StatusIcon status={value} />,
    },
    {
      id: 'name',
      header: 'Name',
      accessor: 'name',
      sortable: true,
      Cell: ({ value, row }) => (
        <button
          onClick={() => setSelectedPack(row.id)}
          className="text-left hover:text-chaos-600 focus:outline-none"
        >
          <div className="font-medium">{value}</div>
          <div className="text-sm text-gray-500 truncate">{row.description}</div>
        </button>
      ),
    },
    {
      id: 'size_bytes',
      header: 'Size',
      accessor: 'size_bytes',
      sortable: true,
      width: 100,
      Cell: ({ value }) => <span>{formatBytes(value)}</span>,
    },
    {
      id: 'file_count',
      header: 'Files',
      accessor: 'file_count',
      sortable: true,
      width: 80,
    },
    {
      id: 'experiments_included',
      header: 'Experiments',
      accessor: 'experiments_included',
      sortable: true,
      width: 100,
    },
    {
      id: 'created_at',
      header: 'Created',
      accessor: 'created_at',
      sortable: true,
      width: 160,
      Cell: ({ value }) => <DateCell value={value} />,
    },
    {
      id: 'actions',
      header: 'Actions',
      accessor: () => null,
      width: 120,
      Cell: ({ row }) => (
        <div className="flex space-x-2">
          {row.status === 'ready' && (
            <a
              href={row.download_url}
              download
              className="inline-flex items-center px-2 py-1 text-xs font-medium text-chaos-700 bg-chaos-100 rounded hover:bg-chaos-200"
            >
              <DocumentArrowDownIcon className="h-4 w-4 mr-1" />
              Download
            </a>
          )}
        </div>
      ),
    },
  ];

  const auditFileColumns: Column<AuditFile>[] = [
    {
      id: 'name',
      header: 'File Name',
      accessor: 'name',
      sortable: true,
      Cell: ({ value, row }) => (
        <div className="flex items-center">
          <DocumentTextIcon className="h-4 w-4 text-gray-400 mr-2" />
          <span className="font-medium">{value}</span>
        </div>
      ),
    },
    {
      id: 'size_bytes',
      header: 'Size',
      accessor: 'size_bytes',
      sortable: true,
      width: 100,
      Cell: ({ value }) => <span>{formatBytes(value)}</span>,
    },
    {
      id: 'mime_type',
      header: 'Type',
      accessor: 'mime_type',
      sortable: true,
      width: 150,
    },
    {
      id: 'checksum',
      header: 'Checksum',
      accessor: 'checksum',
      width: 200,
      Cell: ({ value }) => (
        <code className="text-xs bg-gray-100 px-2 py-1 rounded font-mono">
          {value.slice(0, 16)}...
        </code>
      ),
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

  const selectedPackData = selectedPack ? auditPacks.find(p => p.id === selectedPack) : null;

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Audit Pack Viewer</h1>
          <p className="text-gray-600">
            Download and verify audit packs with cryptographic signatures
          </p>
        </div>
        
        {selectedPack && (
          <button
            onClick={() => setSelectedPack(null)}
            className="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50"
          >
            Back to Packs
          </button>
        )}
      </div>

      {!selectedPack ? (
        /* Audit Packs List */
        <div className="space-y-6">
          {/* Search */}
          <div className="bg-white shadow rounded-lg p-6">
            <div className="flex items-center">
              <MagnifyingGlassIcon className="h-5 w-5 text-gray-400 mr-3" />
              <input
                type="text"
                placeholder="Search audit packs..."
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                className="flex-1 px-3 py-2 border border-gray-300 rounded-md shadow-sm placeholder-gray-400 focus:outline-none focus:ring-chaos-500 focus:border-chaos-500"
              />
            </div>
          </div>

          {/* Packs Table */}
          <div className="bg-white shadow rounded-lg">
            <div className="h-[600px]">
              <VirtualizedTable
                data={filteredPacks}
                columns={auditPackColumns}
                isLoading={isLoadingPacks}
                rowHeight={64}
              />
            </div>
          </div>

          {/* Verification Info */}
          <div className="bg-blue-50 border border-blue-200 rounded-lg p-6">
            <h3 className="text-lg font-medium text-blue-900 mb-2">
              Verification & Compliance
            </h3>
            <div className="text-sm text-blue-800 space-y-2">
              <p>
                • All audit packs include cryptographic signatures for integrity verification
              </p>
              <p>
                • Merkle tree proofs ensure individual file authenticity
              </p>
              <p>
                • Export formats: NDJSON for logs, Parquet for structured data
              </p>
              <p>
                • Use our CLI tool to verify signatures and compare exports
              </p>
            </div>
          </div>
        </div>
      ) : (
        /* Selected Pack Details */
        <div className="space-y-6">
          {/* Pack Info */}
          {selectedPackData && (
            <div className="bg-white shadow rounded-lg p-6">
              <div className="flex items-start justify-between">
                <div className="flex-1">
                  <div className="flex items-center mb-2">
                    <StatusIcon status={selectedPackData.status} />
                    <h2 className="text-xl font-semibold text-gray-900 ml-2">
                      {selectedPackData.name}
                    </h2>
                  </div>
                  <p className="text-gray-600 mb-4">{selectedPackData.description}</p>
                  
                  <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
                    <div>
                      <span className="font-medium text-gray-700">Size:</span>
                      <div>{formatBytes(selectedPackData.size_bytes)}</div>
                    </div>
                    <div>
                      <span className="font-medium text-gray-700">Files:</span>
                      <div>{selectedPackData.file_count}</div>
                    </div>
                    <div>
                      <span className="font-medium text-gray-700">Experiments:</span>
                      <div>{selectedPackData.experiments_included}</div>
                    </div>
                    <div>
                      <span className="font-medium text-gray-700">Created:</span>
                      <div>{new Date(selectedPackData.created_at).toLocaleDateString()}</div>
                    </div>
                  </div>
                </div>
                
                {selectedPackData.status === 'ready' && (
                  <a
                    href={selectedPackData.download_url}
                    download
                    className="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-chaos-600 hover:bg-chaos-700"
                  >
                    <DocumentArrowDownIcon className="h-4 w-4 mr-2" />
                    Download Pack
                  </a>
                )}
              </div>

              {/* Cryptographic Info */}
              {selectedPackData.signature && (
                <div className="mt-6 pt-6 border-t border-gray-200">
                  <h3 className="text-sm font-medium text-gray-700 mb-3">
                    Cryptographic Verification
                  </h3>
                  <div className="space-y-2 text-xs font-mono">
                    <div>
                      <span className="text-gray-500">Signature:</span>
                      <div className="bg-gray-100 p-2 rounded break-all">
                        {selectedPackData.signature}
                      </div>
                    </div>
                    <div>
                      <span className="text-gray-500">Merkle Root:</span>
                      <div className="bg-gray-100 p-2 rounded break-all">
                        {selectedPackData.merkle_root}
                      </div>
                    </div>
                  </div>
                </div>
              )}
            </div>
          )}

          {/* Files Table */}
          <div className="bg-white shadow rounded-lg">
            <div className="px-6 py-4 border-b border-gray-200">
              <h3 className="text-lg font-medium text-gray-900">Pack Contents</h3>
            </div>
            <div className="h-[500px]">
              <VirtualizedTable
                data={auditFiles}
                columns={auditFileColumns}
                isLoading={isLoadingFiles}
                rowHeight={56}
              />
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
