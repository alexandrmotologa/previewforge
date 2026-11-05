import React from 'react';
import { Layers, Play, Settings, RefreshCw } from 'lucide-react';
import { SystemStats, SystemConfig } from '../types';

interface HeaderProps {
  stats: SystemStats | null;
  config: SystemConfig | null;
  onOpenSimulator: () => void;
  onOpenSettings: () => void;
  onRefresh: () => void;
  isRefreshing: boolean;
}

export const Header: React.FC<HeaderProps> = ({
  stats,
  config,
  onOpenSimulator,
  onOpenSettings,
  onRefresh,
  isRefreshing,
}) => {
  const formatUptime = (seconds: number) => {
    const hours = Math.floor(seconds / 3600);
    const mins = Math.floor((seconds % 3600) / 60);
    if (hours > 0) return `${hours}h ${mins}m`;
    return `${mins}m ${seconds % 60}s`;
  };

  return (
    <header className="border-b border-slate-800 bg-slate-900/60 backdrop-blur-md sticky top-0 z-30">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
        {/* Brand */}
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 rounded-lg bg-emerald-500/10 border border-emerald-500/30 flex items-center justify-center text-emerald-400">
            <Layers className="w-5 h-5" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <span className="font-semibold text-slate-100 tracking-tight text-lg">PreviewForge</span>
              <span className="px-2 py-0.5 rounded text-[11px] font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                v1.0.0
              </span>
            </div>
            <p className="text-xs text-slate-400 hidden sm:block">Self-hosted ephemeral PR previews</p>
          </div>
        </div>

        {/* Live Status Indicators */}
        <div className="flex items-center gap-2 sm:gap-4">
          {stats && (
            <div className="hidden md:flex items-center gap-3 px-3 py-1.5 rounded-md bg-slate-800/60 border border-slate-700/60 text-xs text-slate-300">
              <div className="flex items-center gap-1.5">
                <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse" />
                <span>Online</span>
              </div>
              <span className="text-slate-600">|</span>
              <span>Uptime: {formatUptime(stats.daemon_uptime_sec)}</span>
              <span className="text-slate-600">|</span>
              <span>Port: {config?.public_port || 9850}</span>
            </div>
          )}

          {/* Action buttons */}
          <button
            onClick={onRefresh}
            title="Refresh deployments"
            className="p-2 rounded-lg bg-slate-800 hover:bg-slate-700 border border-slate-700 text-slate-300 hover:text-white transition-colors"
          >
            <RefreshCw className={`w-4 h-4 ${isRefreshing ? 'animate-spin' : ''}`} />
          </button>

          <button
            onClick={onOpenSimulator}
            className="flex items-center gap-1.5 px-3 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-medium shadow-sm transition-colors"
          >
            <Play className="w-3.5 h-3.5" />
            <span className="hidden sm:inline">Simulate Deploy</span>
          </button>

          <button
            onClick={onOpenSettings}
            className="p-2 rounded-lg bg-slate-800 hover:bg-slate-700 border border-slate-700 text-slate-300 hover:text-white transition-colors"
            title="System Settings"
          >
            <Settings className="w-4 h-4" />
          </button>
        </div>
      </div>
    </header>
  );
};
