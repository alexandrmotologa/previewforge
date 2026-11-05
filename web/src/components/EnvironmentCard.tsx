import React, { useState } from 'react';
import { ExternalLink, Terminal, RotateCw, Trash2, GitBranch, GitCommit, Check, Clock, AlertTriangle } from 'lucide-react';
import { Deployment } from '../types';

interface EnvironmentCardProps {
  deployment: Deployment;
  onViewLogs: (dep: Deployment) => void;
  onRebuild: (id: string) => void;
  onDestroy: (id: string) => void;
}

export const EnvironmentCard: React.FC<EnvironmentCardProps> = ({
  deployment,
  onViewLogs,
  onRebuild,
  onDestroy,
}) => {
  const [copiedSha, setCopiedSha] = useState(false);

  const copySha = () => {
    navigator.clipboard.writeText(deployment.commit_sha);
    setCopiedSha(true);
    setTimeout(() => setCopiedSha(false), 1500);
  };

  const getStatusBadge = () => {
    switch (deployment.status) {
      case 'running':
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
            <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse" />
            Live
          </span>
        );
      case 'building':
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium bg-amber-500/10 text-amber-400 border border-amber-500/20">
            <RotateCw className="w-3 h-3 animate-spin" />
            Building
          </span>
        );
      case 'failed':
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium bg-rose-500/10 text-rose-400 border border-rose-500/20">
            <AlertTriangle className="w-3 h-3" />
            Failed
          </span>
        );
      case 'stopped':
      case 'destroyed':
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium bg-slate-800 text-slate-400 border border-slate-700">
            Closed
          </span>
        );
      default:
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium bg-slate-800 text-slate-400 border border-slate-700">
            Pending
          </span>
        );
    }
  };

  return (
    <div className="bg-slate-900/60 border border-slate-800 hover:border-slate-700/80 rounded-xl p-5 transition-all flex flex-col justify-between">
      <div>
        {/* Top Header: PR & Status */}
        <div className="flex items-start justify-between gap-3 mb-3">
          <div>
            <div className="flex items-center gap-2">
              <span className="text-sm font-semibold text-emerald-400">PR #{deployment.pr_number}</span>
              <span className="text-xs text-slate-400">({deployment.repo_owner}/{deployment.repo_name})</span>
            </div>
            <h3 className="text-base font-medium text-slate-100 mt-1 line-clamp-1" title={deployment.pr_title}>
              {deployment.pr_title || `Pull Request #${deployment.pr_number}`}
            </h3>
          </div>
          <div>{getStatusBadge()}</div>
        </div>

        {/* Branch and Commit info */}
        <div className="flex flex-wrap items-center gap-3 text-xs text-slate-400 mb-4 bg-slate-950/40 p-2.5 rounded-lg border border-slate-800/80">
          <div className="flex items-center gap-1.5">
            <GitBranch className="w-3.5 h-3.5 text-slate-500" />
            <span className="font-mono text-slate-300">{deployment.branch}</span>
          </div>
          <span className="text-slate-700">•</span>
          <button
            onClick={copySha}
            className="flex items-center gap-1 hover:text-slate-200 transition-colors"
            title="Copy commit SHA"
          >
            <GitCommit className="w-3.5 h-3.5 text-slate-500" />
            <span className="font-mono">{deployment.commit_sha ? deployment.commit_sha.slice(0, 7) : 'latest'}</span>
            {copiedSha && <Check className="w-3 h-3 text-emerald-400 ml-0.5" />}
          </button>
          <span className="text-slate-700">•</span>
          <div className="flex items-center gap-1">
            <Clock className="w-3.5 h-3.5 text-slate-500" />
            <span>{deployment.build_duration_sec}s</span>
          </div>
          <span className="text-slate-700">•</span>
          <div className="text-slate-500">
            Port: <span className="font-mono text-slate-300">{deployment.port}</span>
          </div>
        </div>

        {/* Error message callout if failed */}
        {deployment.error_message && (
          <div className="mb-4 p-2.5 rounded-lg bg-rose-500/10 border border-rose-500/20 text-rose-300 text-xs font-mono line-clamp-2">
            {deployment.error_message}
          </div>
        )}

        {/* Live Subdomain Link */}
        {deployment.status === 'running' && (
          <div className="mb-4">
            <a
              href={deployment.preview_url}
              target="_blank"
              rel="noreferrer"
              className="inline-flex items-center gap-1.5 text-xs font-medium text-emerald-400 hover:text-emerald-300 hover:underline break-all"
            >
              <span>{deployment.preview_url}</span>
              <ExternalLink className="w-3.5 h-3.5 shrink-0" />
            </a>
          </div>
        )}
      </div>

      {/* Action Footer */}
      <div className="pt-3 border-t border-slate-800/80 flex items-center justify-between gap-2 mt-2">
        <div className="flex items-center gap-2">
          {deployment.status === 'running' && (
            <a
              href={deployment.preview_url}
              target="_blank"
              rel="noreferrer"
              className="flex items-center gap-1 px-2.5 py-1.5 rounded-md bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-medium transition-colors"
            >
              <ExternalLink className="w-3.5 h-3.5" />
              <span>Open</span>
            </a>
          )}
          <button
            onClick={() => onViewLogs(deployment)}
            className="flex items-center gap-1 px-2.5 py-1.5 rounded-md bg-slate-800 hover:bg-slate-700 border border-slate-700 text-slate-300 hover:text-white text-xs font-medium transition-colors"
          >
            <Terminal className="w-3.5 h-3.5" />
            <span>Logs</span>
          </button>
        </div>

        <div className="flex items-center gap-1">
          <button
            onClick={() => onRebuild(deployment.id)}
            title="Rebuild stack"
            className="p-1.5 rounded-md hover:bg-slate-800 text-slate-400 hover:text-slate-200 transition-colors"
          >
            <RotateCw className="w-3.5 h-3.5" />
          </button>
          <button
            onClick={() => onDestroy(deployment.id)}
            title="Destroy stack and reclaim resources"
            className="p-1.5 rounded-md hover:bg-rose-500/10 text-slate-400 hover:text-rose-400 transition-colors"
          >
            <Trash2 className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>
    </div>
  );
};
