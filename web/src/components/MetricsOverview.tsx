import React from 'react';
import { Box, Network, Cpu, HardDrive, Moon, AlertTriangle } from 'lucide-react';
import { SystemStats } from '../types';

interface MetricsOverviewProps {
  stats: SystemStats | null;
}

export const MetricsOverview: React.FC<MetricsOverviewProps> = ({ stats }) => {
  if (!stats) return null;

  return (
    <div className="mb-8">
      {stats.disk_low_alert && (
        <div className="mb-4 p-3 rounded-xl bg-amber-500/10 border border-amber-500/30 flex items-center gap-3 text-amber-300 text-xs">
          <AlertTriangle className="w-5 h-5 shrink-0 text-amber-400" />
          <div>
            <strong>VPS Disk Alert:</strong> Available host storage is running low ({stats.host_disk_free_gb.toFixed(1)} GB free).
            PreviewForge will automatically prune older build cache and dangling images.
          </div>
        </div>
      )}

      <div className="grid grid-cols-2 md:grid-cols-4 gap-3 sm:gap-4">
        {/* Active & Hibernated Previews */}
        <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-4">
          <div className="flex items-center justify-between mb-2">
            <span className="text-xs font-medium text-slate-400">Environments</span>
            <Box className="w-4 h-4 text-emerald-400" />
          </div>
          <div className="flex items-baseline gap-2">
            <span className="text-2xl font-bold text-slate-100">{stats.active_deployments}</span>
            <span className="text-xs text-emerald-400 font-medium">live</span>
            {stats.sleeping_deployments > 0 && (
              <span className="inline-flex items-center gap-1 text-xs text-amber-400 font-medium ml-1">
                <Moon className="w-3 h-3" />
                {stats.sleeping_deployments} sleeping
              </span>
            )}
          </div>
          <p className="text-[11px] text-slate-500 mt-1">{stats.total_deployments} total recorded</p>
        </div>

        {/* Multiplexed Ports */}
        <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-4">
          <div className="flex items-center justify-between mb-2">
            <span className="text-xs font-medium text-slate-400">Multiplexed Ports</span>
            <Network className="w-4 h-4 text-sky-400" />
          </div>
          <div className="flex items-baseline gap-2">
            <span className="text-2xl font-bold text-slate-100">{stats.port_pool_used}</span>
            <span className="text-xs text-slate-500">/ {stats.port_pool_total} assigned</span>
          </div>
          <p className="text-[11px] text-slate-500 mt-1">Host dynamic range 31000+</p>
        </div>

        {/* Host Memory */}
        <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-4">
          <div className="flex items-center justify-between mb-2">
            <span className="text-xs font-medium text-slate-400">Daemon RAM</span>
            <Cpu className="w-4 h-4 text-violet-400" />
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

        {/* Host Disk */}
        <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-4">
          <div className="flex items-center justify-between mb-2">
            <span className="text-xs font-medium text-slate-400">Host Storage</span>
            <HardDrive className={`w-4 h-4 ${stats.disk_low_alert ? 'text-amber-400' : 'text-emerald-400'}`} />
          </div>
          <div className="flex items-baseline gap-2">
            <span className="text-2xl font-bold text-slate-100">{stats.host_disk_percent.toFixed(0)}%</span>
            <span className="text-xs text-slate-500">{stats.host_disk_free_gb.toFixed(1)} GB free</span>
          </div>
          <div className="w-full bg-slate-800 rounded-full h-1.5 mt-2">
            <div
              className={`h-1.5 rounded-full ${stats.disk_low_alert ? 'bg-amber-400' : 'bg-emerald-400'}`}
              style={{ width: `${Math.min(stats.host_disk_percent, 100)}%` }}
            />
          </div>
        </div>
      </div>
    </div>
  );
};
