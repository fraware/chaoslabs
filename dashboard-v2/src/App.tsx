import React, { Suspense } from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import { ErrorBoundary } from './components/ErrorBoundary';
import { Layout } from './components/Layout';
import { LoadingSpinner } from './components/LoadingSpinner';

// Lazy load components for code splitting
const Dashboard = React.lazy(() => import('./pages/Dashboard'));
const ExperimentsList = React.lazy(() => import('./pages/ExperimentsList'));
const ExperimentDetail = React.lazy(() => import('./pages/ExperimentDetail'));
const AuditPack = React.lazy(() => import('./pages/AuditPack'));
const Settings = React.lazy(() => import('./pages/Settings'));
const NotFound = React.lazy(() => import('./pages/NotFound'));

function App() {
  return (
    <ErrorBoundary>
      <Layout>
        <Suspense fallback={<LoadingSpinner />}>
          <Routes>
            <Route path="/" element={<Navigate to="/dashboard" replace />} />
            <Route path="/dashboard" element={<Dashboard />} />
            <Route path="/experiments" element={<ExperimentsList />} />
            <Route path="/experiments/:id" element={<ExperimentDetail />} />
            <Route path="/audit-pack" element={<AuditPack />} />
            <Route path="/settings" element={<Settings />} />
            <Route path="*" element={<NotFound />} />
          </Routes>
        </Suspense>
      </Layout>
    </ErrorBoundary>
  );
}

export default App;
