import React, { useState } from 'react';
import { EnvironmentCard } from './EnvironmentCard';
import { Deployment } from '../types';
import { Layers } from 'lucide-react';

interface EnvironmentGridProps {
  deployments: Deployment[];
  onViewLogs: (dep: Deployment) => void;
  onRebuild: (id: string) => void;
  onDestroy: (id: string) => void;
}

export const EnvironmentGrid: React.FC<EnvironmentGridProps> = ({
  deployments,
  onViewLogs,
  onRebuild,
  onDestroy,
}) => {
  const [filter, setFilter] = useState<'all' | 'active' | 'closed'>('all');
  const [search, setSearch] = useState('');

  const filteredDeployments = deployments.filter((d) => {
    // Status filter
    if (filter === 'active') {
      if (d.status !== 'running' && d.status !== 'building') return false;
    } else if (filter === 'closed') {
      if (d.status !== 'stopped' && d.status !== 'destroyed' && d.status !== 'failed') return false;
    }

    // Search query
    if (search.trim() !== '') {
      const q = search.toLowerCase();
      const match =
        d.pr_title?.toLowerCase().includes(q) ||
        d.branch?.toLowerCase().includes(q) ||
        d.repo_name?.toLowerCase().includes(q) ||
        String(d.pr_number).includes(q);
      if (!match) return false;
    }

    return true;
  });

  return (
    <div>
      {/* Controls: Search and Filter Tabs */}
      <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 mb-6">
        <div className="flex items-center gap-1 p-1 bg-slate-900 border border-slate-800 rounded-lg self-start">
          <button
            onClick={() => setFilter('all')}
            className={`px-3 py-1.5 rounded-md text-xs font-medium transition-colors ${
              filter === 'all'
                ? 'bg-slate-800 text-white shadow-sm'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            All ({deployments.length})
          </button>
          <button
            onClick={() => setFilter('active')}
            className={`px-3 py-1.5 rounded-md text-xs font-medium transition-colors ${
              filter === 'active'
                ? 'bg-slate-800 text-white shadow-sm'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            Active ({deployments.filter((d) => d.status === 'running' || d.status === 'building').length})
          </button>
          <button
            onClick={() => setFilter('closed')}
            className={`px-3 py-1.5 rounded-md text-xs font-medium transition-colors ${
              filter === 'closed'
                ? 'bg-slate-800 text-white shadow-sm'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            Closed ({deployments.filter((d) => d.status === 'stopped' || d.status === 'destroyed' || d.status === 'failed').length})
          </button>
        </div>

        <div className="relative">
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search by branch, PR # or repo..."
            className="w-full sm:w-64 bg-slate-900/80 border border-slate-800 rounded-lg px-3 py-1.5 text-xs text-slate-200 placeholder-slate-500 focus:outline-none focus:border-emerald-500/50"
          />
        </div>
      </div>

      {/* Grid or Empty State */}
      {filteredDeployments.length === 0 ? (
        <div className="border border-dashed border-slate-800 rounded-2xl p-12 text-center bg-slate-900/30">
          <div className="w-12 h-12 rounded-xl bg-slate-800/80 border border-slate-700/60 flex items-center justify-center text-slate-400 mx-auto mb-3">
            <Layers className="w-6 h-6" />
          </div>
          <h3 className="text-sm font-semibold text-slate-200 mb-1">No preview environments found</h3>
          <p className="text-xs text-slate-400 max-w-sm mx-auto mb-4">
            Pull requests with open status will automatically trigger isolated preview container deployments.
          </p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {filteredDeployments.map((dep) => (
            <EnvironmentCard
              key={dep.id}
              deployment={dep}
              onViewLogs={onViewLogs}
              onRebuild={onRebuild}
              onDestroy={onDestroy}
            />
          ))}
        </div>
      )}
    </div>
  );
};
