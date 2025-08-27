import React from 'react';
import { useParams } from 'react-router-dom';

export default function ExperimentDetail() {
  const { id } = useParams();
  
  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-gray-900">Experiment Detail</h1>
      <p>Experiment ID: {id}</p>
      <div className="bg-white p-6 rounded-lg shadow">
        <p className="text-gray-600">Detailed experiment view coming soon...</p>
      </div>
    </div>
  );
}
