import React from 'react';
import { Box, Network, Cpu, HardDrive } from 'lucide-react';
import { SystemStats } from '../types';

interface MetricsOverviewProps {
  stats: SystemStats | null;
}

export const MetricsOverview: React.FC<MetricsOverviewProps> = ({ stats }) => {
  if (!stats) return null;

  return (
    <div className="grid grid-cols-2 md:grid-cols-4 gap-3 sm:gap-4 mb-8">
      {/* Active Previews */}
      <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-4">
        <div className="flex items-center justify-between mb-2">
          <span className="text-xs font-medium text-slate-400">Active Previews</span>
          <Box className="w-4 h-4 text-emerald-400" />
        </div>
        <div className="flex items-baseline gap-2">
          <span className="text-2xl font-bold text-slate-100">{stats.active_deployments}</span>
          <span className="text-xs text-slate-500">/ {stats.total_deployments} total</span>
        </div>
        <p className="text-[11px] text-slate-500 mt-1">Isolated compose stacks</p>
      </div>

      {/* Port Pool */}
      <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-4">
        <div className="flex items-center justify-between mb-2">
          <span className="text-xs font-medium text-slate-400">Multiplexed Ports</span>
          <Network className="w-4 h-4 text-sky-400" />
        </div>
        <div className="flex items-baseline gap-2">
          <span className="text-2xl font-bold text-slate-100">{stats.port_pool_used}</span>
          <span className="text-xs text-slate-500">/ {stats.port_pool_total} free</span>
        </div>
        <p className="text-[11px] text-slate-500 mt-1">Host dynamic range 31000+</p>
      </div>

      {/* Host RAM */}
      <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-4">
        <div className="flex items-center justify-between mb-2">
          <span className="text-xs font-medium text-slate-400">Process Memory</span>
          <HardDrive className="w-4 h-4 text-violet-400" />
        </div>
        <div className="flex items-baseline gap-2">
          <span className="text-2xl font-bold text-slate-100">{stats.host_ram_used_mb.toFixed(1)}</span>
          <span className="text-xs text-slate-500">MB</span>
        </div>
        <div className="w-full bg-slate-800 rounded-full h-1.5 mt-2">
          <div
            className="bg-violet-400 h-1.5 rounded-full"
            style={{ width: `${Math.min(stats.host_ram_percent, 100)}%` }}
          />
        </div>
      </div>

      {/* Host CPU */}
      <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-4">
        <div className="flex items-center justify-between mb-2">
          <span className="text-xs font-medium text-slate-400">Host CPU</span>
          <Cpu className="w-4 h-4 text-amber-400" />
        </div>
        <div className="flex items-baseline gap-2">
          <span className="text-2xl font-bold text-slate-100">{stats.host_cpu_percent.toFixed(1)}%</span>
          <span className="text-xs text-slate-500">supervisor load</span>
        </div>
        <div className="w-full bg-slate-800 rounded-full h-1.5 mt-2">
          <div
            className="bg-amber-400 h-1.5 rounded-full"
            style={{ width: `${Math.min(stats.host_cpu_percent, 100)}%` }}
          />
        </div>
      </div>
    </div>
  );
};
