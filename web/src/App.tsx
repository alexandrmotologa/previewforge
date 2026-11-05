import React, { useState, useEffect, useCallback } from 'react';
import { Header } from './components/Header';
import { MetricsOverview } from './components/MetricsOverview';
import { EnvironmentGrid } from './components/EnvironmentGrid';
import { LogDrawer } from './components/LogDrawer';
import { SimulatorModal } from './components/SimulatorModal';
import { SettingsModal } from './components/SettingsModal';
import { Deployment, SystemStats, SystemConfig } from './types';

export const App: React.FC = () => {
  const [deployments, setDeployments] = useState<Deployment[]>([]);
  const [stats, setStats] = useState<SystemStats | null>(null);
  const [config, setConfig] = useState<SystemConfig | null>(null);
  const [selectedDepForLogs, setSelectedDepForLogs] = useState<Deployment | null>(null);
  const [drawerTab, setDrawerTab] = useState<'logs' | 'exec'>('logs');
  const [isSimulatorOpen, setIsSimulatorOpen] = useState(false);
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);
  const [isRefreshing, setIsRefreshing] = useState(false);

  const fetchDeployments = useCallback(async () => {
    try {
      const res = await fetch('/api/deployments');
      if (res.ok) {
        const data = await res.json();
        setDeployments(data);
      }
    } catch (err) {
      console.error('Failed to fetch deployments:', err);
    }
  }, []);

  const fetchStats = useCallback(async () => {
    try {
      const res = await fetch('/api/system/stats');
      if (res.ok) {
        const data = await res.json();
        setStats(data);
      }
    } catch (err) {
      console.error('Failed to fetch system stats:', err);
    }
  }, []);

  const fetchConfig = useCallback(async () => {
    try {
      const res = await fetch('/api/system/config');
      if (res.ok) {
        const data = await res.json();
        setConfig(data);
      }
    } catch (err) {
      console.error('Failed to fetch system config:', err);
    }
  }, []);

  const refreshAll = useCallback(async () => {
    setIsRefreshing(true);
    await Promise.all([fetchDeployments(), fetchStats(), fetchConfig()]);
    setTimeout(() => setIsRefreshing(false), 400);
  }, [fetchDeployments, fetchStats, fetchConfig]);

  useEffect(() => {
    refreshAll();
    const interval = setInterval(() => {
      fetchDeployments();
      fetchStats();
    }, 4000);
    return () => clearInterval(interval);
  }, [refreshAll, fetchDeployments, fetchStats]);

  const handleRebuild = async (id: string) => {
    try {
      await fetch(`/api/deployments/${id}/rebuild`, { method: 'POST' });
      fetchDeployments();
    } catch (err) {
      console.error('Failed to trigger rebuild:', err);
    }
  };

  const handleSleep = async (id: string) => {
    try {
      await fetch(`/api/deployments/${id}/sleep`, { method: 'POST' });
      fetchDeployments();
    } catch (err) {
      console.error('Failed to trigger sleep:', err);
    }
  };

  const handleWake = async (id: string) => {
    try {
      await fetch(`/api/deployments/${id}/wake`, { method: 'POST' });
      fetchDeployments();
    } catch (err) {
      console.error('Failed to trigger wake:', err);
    }
  };

  const handleDestroy = async (id: string) => {
    if (!window.confirm('Are you sure you want to destroy this preview environment? All container volumes will be deleted.')) {
      return;
    }
    try {
      await fetch(`/api/deployments/${id}`, { method: 'DELETE' });
      fetchDeployments();
    } catch (err) {
      console.error('Failed to tear down deployment:', err);
    }
  };

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex flex-col font-sans">
      <Header
        stats={stats}
        config={config}
        onOpenSimulator={() => setIsSimulatorOpen(true)}
        onOpenSettings={() => setIsSettingsOpen(true)}
        onRefresh={refreshAll}
        isRefreshing={isRefreshing}
      />

      <main className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <MetricsOverview stats={stats} />

        <div className="mb-4 flex items-center justify-between">
          <div>
            <h2 className="text-lg font-semibold text-slate-100">Pull Request Environments</h2>
            <p className="text-xs text-slate-400">
              Ephemeral Docker Compose preview stacks with isolated port multiplexing &amp; auto-hibernation
            </p>
          </div>
        </div>

        <EnvironmentGrid
          deployments={deployments}
          onViewLogs={(dep, initialTab) => {
            setSelectedDepForLogs(dep);
            setDrawerTab(initialTab || 'logs');
          }}
          onRebuild={handleRebuild}
          onDestroy={handleDestroy}
          onSleep={handleSleep}
          onWake={handleWake}
        />
      </main>

      <footer className="border-t border-slate-900 bg-slate-950 py-6 mt-12 text-center text-xs text-slate-500">
        <p>PreviewForge Ephemeral Preview Daemon • Free &amp; self-hosted alternative to Vercel Previews</p>
      </footer>

      {/* Slide-over log & exec drawer */}
      <LogDrawer
        deployment={selectedDepForLogs}
        initialTab={drawerTab}
        onClose={() => setSelectedDepForLogs(null)}
      />

      {/* Simulator Modal */}
      <SimulatorModal
        isOpen={isSimulatorOpen}
        onClose={() => setIsSimulatorOpen(false)}
        onSuccess={() => {
          fetchDeployments();
          fetchStats();
        }}
      />

      {/* Settings Modal */}
      <SettingsModal
        isOpen={isSettingsOpen}
        config={config}
        onClose={() => setIsSettingsOpen(false)}
      />
    </div>
  );
};
