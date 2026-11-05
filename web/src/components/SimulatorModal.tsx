import React, { useState } from 'react';
import { X, Play, Sparkles } from 'lucide-react';

interface SimulatorModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

export const SimulatorModal: React.FC<SimulatorModalProps> = ({ isOpen, onClose, onSuccess }) => {
  const [prNumber, setPrNumber] = useState<number>(42);
  const [prTitle, setPrTitle] = useState('Add Stripe Checkout & Webhook Handlers');
  const [repoOwner, setRepoOwner] = useState('alexandrmotologa');
  const [repoName, setRepoName] = useState('ecommerce-store');
  const [branch, setBranch] = useState('feat/stripe-checkout');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError('');

    try {
      const res = await fetch('/api/simulator/deploy', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          pr_number: Number(prNumber),
          pr_title: prTitle,
          repo_owner: repoOwner,
          repo_name: repoName,
          branch: branch,
          commit_sha: 'a' + Math.random().toString(16).substring(2, 9),
          author: 'alexandrmotologa',
        }),
      });

      if (!res.ok) {
        throw new Error(`Failed to queue test deployment: HTTP ${res.status}`);
      }

      onSuccess();
      onClose();
    } catch (err: any) {
      setError(err.message || 'An error occurred');
    } finally {
      setLoading(false);
    }
  };

  const setPreset = (preset: 'nextjs' | 'go' | 'node') => {
    if (preset === 'nextjs') {
      setPrNumber(104);
      setPrTitle('Migrate to React 19 and Server Actions');
      setRepoName('saas-portal');
      setBranch('feat/react-19');
    } else if (preset === 'go') {
      setPrNumber(58);
      setPrTitle('Implement Redis Session Cache');
      setRepoName('api-gateway');
      setBranch('perf/redis-cache');
    } else if (preset === 'node') {
      setPrNumber(23);
      setPrTitle('Upgrade Postgres connection pooling');
      setRepoName('backend-core');
      setBranch('fix/db-pool');
    }
  };

  return (
    <div className="fixed inset-0 z-50 overflow-y-auto bg-slate-950/70 backdrop-blur-sm flex items-center justify-center p-4">
      <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-lg shadow-2xl p-6">
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2">
            <div className="p-2 rounded-lg bg-emerald-500/10 text-emerald-400">
              <Sparkles className="w-4 h-4" />
            </div>
            <h2 className="text-base font-semibold text-slate-100">Simulate PR Preview Deploy</h2>
          </div>
          <button onClick={onClose} className="p-1 rounded-md text-slate-400 hover:text-white">
            <X className="w-5 h-5" />
          </button>
        </div>

        <p className="text-xs text-slate-400 mb-4">
          Trigger an ephemeral preview deployment without waiting for a real GitHub webhook event. Useful for testing port multiplexing and routing.
        </p>

        {/* Quick Presets */}
        <div className="flex items-center gap-2 mb-4">
          <span className="text-[11px] text-slate-500">Presets:</span>
          <button
            type="button"
            onClick={() => setPreset('nextjs')}
            className="px-2 py-0.5 rounded text-[11px] bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700"
          >
            Next.js App
          </button>
          <button
            type="button"
            onClick={() => setPreset('go')}
            className="px-2 py-0.5 rounded text-[11px] bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700"
          >
            Go API
          </button>
          <button
            type="button"
            onClick={() => setPreset('node')}
            className="px-2 py-0.5 rounded text-[11px] bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700"
          >
            Node Backend
          </button>
        </div>

        {error && (
          <div className="mb-4 p-3 rounded-lg bg-rose-500/10 border border-rose-500/20 text-rose-300 text-xs">
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="grid grid-cols-3 gap-3">
            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1">PR Number</label>
              <input
                type="number"
                required
                value={prNumber}
                onChange={(e) => setPrNumber(Number(e.target.value))}
                className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs text-slate-100 focus:outline-none focus:border-emerald-500/50"
              />
            </div>
            <div className="col-span-2">
              <label className="block text-xs font-medium text-slate-300 mb-1">Repository Name</label>
              <input
                type="text"
                required
                value={repoName}
                onChange={(e) => setRepoName(e.target.value)}
                className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs text-slate-100 focus:outline-none focus:border-emerald-500/50"
              />
            </div>
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">PR Title</label>
            <input
              type="text"
              required
              value={prTitle}
              onChange={(e) => setPrTitle(e.target.value)}
              className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs text-slate-100 focus:outline-none focus:border-emerald-500/50"
            />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1">Repo Owner</label>
              <input
                type="text"
                required
                value={repoOwner}
                onChange={(e) => setRepoOwner(e.target.value)}
                className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs text-slate-100 focus:outline-none focus:border-emerald-500/50"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1">Branch Ref</label>
              <input
                type="text"
                required
                value={branch}
                onChange={(e) => setBranch(e.target.value)}
                className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs text-slate-100 focus:outline-none focus:border-emerald-500/50"
              />
            </div>
          </div>

          <div className="pt-2 flex justify-end gap-2">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-medium transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={loading}
              className="flex items-center gap-1.5 px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-medium transition-colors disabled:opacity-50"
            >
              <Play className="w-3.5 h-3.5" />
              <span>{loading ? 'Queuing...' : 'Deploy Preview'}</span>
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
