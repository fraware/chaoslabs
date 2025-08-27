import React from 'react';
import { Link } from 'react-router-dom';

export default function NotFound() {
  return (
    <div className="text-center py-12">
      <h1 className="text-4xl font-bold text-gray-900 mb-4">404</h1>
      <p className="text-gray-600 mb-8">Page not found</p>
      <Link
        to="/dashboard"
        className="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-chaos-600 hover:bg-chaos-700"
      >
        Go to Dashboard
      </Link>
    </div>
  );
}
