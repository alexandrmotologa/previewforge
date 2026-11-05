import React, { useEffect, useRef, useState } from 'react';
import { X, Copy, Check, ArrowDown, Search, Terminal } from 'lucide-react';
import { Deployment } from '../types';

interface LogDrawerProps {
  deployment: Deployment | null;
  onClose: () => void;
}

export const LogDrawer: React.FC<LogDrawerProps> = ({ deployment, onClose }) => {
  const [logs, setLogs] = useState<string[]>([]);
  const [isConnected, setIsConnected] = useState(false);
  const [autoScroll, setAutoScroll] = useState(true);
  const [copied, setCopied] = useState(false);
  const [filter, setFilter] = useState('');
  const logContainerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!deployment) {
      setLogs([]);
      return;
    }

    setLogs([]);
    setIsConnected(false);

    // Initial fetch of historical logs
    fetch(`/api/deployments/${deployment.id}/logs`)
      .then((res) => res.json())
      .then((data) => {
        if (data.logs && Array.isArray(data.logs)) {
          setLogs(data.logs);
        }
      })
      .catch((err) => console.error('Failed to load initial logs:', err));

    // Connect SSE stream
    const eventSource = new EventSource(`/api/deployments/${deployment.id}/logs/stream`);

    eventSource.onopen = () => {
      setIsConnected(true);
    };

    eventSource.onmessage = (event) => {
      if (event.data) {
        setLogs((prev) => {
          // Avoid duplicate lines if stream replays initial history
          if (prev.length > 0 && prev[prev.length - 1] === event.data) {
            return prev;
          }
          return [...prev, event.data];
        });
      }
    };

    eventSource.onerror = () => {
      setIsConnected(false);
    };

    return () => {
      eventSource.close();
    };
  }, [deployment]);

  // Auto-scroll to bottom
  useEffect(() => {
    if (autoScroll && logContainerRef.current) {
      logContainerRef.current.scrollTop = logContainerRef.current.scrollHeight;
    }
  }, [logs, autoScroll]);

  const copyLogs = () => {
    navigator.clipboard.writeText(logs.join('\n'));
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  if (!deployment) return null;

  const filteredLogs = filter.trim()
    ? logs.filter((line) => line.toLowerCase().includes(filter.toLowerCase()))
    : logs;

  return (
    <div className="fixed inset-0 z-50 overflow-hidden bg-slate-950/70 backdrop-blur-sm flex justify-end">
      <div className="w-full max-w-3xl bg-slate-900 border-l border-slate-800 h-full flex flex-col shadow-2xl">
        {/* Drawer Header */}
        <div className="p-4 border-b border-slate-800 flex items-center justify-between">
          <div className="flex items-center gap-2.5">
            <div className="p-1.5 rounded bg-slate-800 text-emerald-400">
              <Terminal className="w-4 h-4" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <span className="text-sm font-semibold text-slate-100">
                  Build Logs: PR #{deployment.pr_number}
                </span>
                <span
                  className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-[10px] font-medium ${
                    isConnected
                      ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                      : 'bg-slate-800 text-slate-400'
                  }`}
                >
                  <span
                    className={`w-1.5 h-1.5 rounded-full ${
                      isConnected ? 'bg-emerald-400 animate-pulse' : 'bg-slate-500'
                    }`}
                  />
                  {isConnected ? 'Live SSE' : 'Offline'}
                </span>
              </div>
              <p className="text-xs text-slate-400 font-mono">
                {deployment.repo_owner}/{deployment.repo_name} ({deployment.branch})
              </p>
            </div>
          </div>

          <div className="flex items-center gap-2">
            <button
              onClick={copyLogs}
              className="flex items-center gap-1 px-2.5 py-1.5 rounded-md bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-medium border border-slate-700 transition-colors"
            >
              {copied ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
              <span>{copied ? 'Copied' : 'Copy'}</span>
            </button>
            <button
              onClick={onClose}
              className="p-1.5 rounded-md hover:bg-slate-800 text-slate-400 hover:text-white transition-colors"
            >
              <X className="w-5 h-5" />
            </button>
          </div>
        </div>

        {/* Toolbar: Filter and Autoscroll */}
        <div className="px-4 py-2 border-b border-slate-800/80 bg-slate-950/40 flex items-center justify-between gap-3 text-xs">
          <div className="relative flex-1 max-w-sm">
            <Search className="w-3.5 h-3.5 text-slate-500 absolute left-2.5 top-1/2 -translate-y-1/2" />
            <input
              type="text"
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              placeholder="Filter log lines..."
              className="w-full bg-slate-900 border border-slate-800 rounded px-2.5 py-1 pl-8 text-xs text-slate-200 placeholder-slate-500 focus:outline-none focus:border-emerald-500/50"
            />
          </div>

          <button
            onClick={() => setAutoScroll(!autoScroll)}
            className={`flex items-center gap-1.5 px-2.5 py-1 rounded transition-colors ${
              autoScroll
                ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                : 'text-slate-400 hover:text-slate-200'
            }`}
          >
            <ArrowDown className="w-3.5 h-3.5" />
            <span>Auto-scroll</span>
          </button>
        </div>

        {/* Terminal Content */}
        <div
          ref={logContainerRef}
          className="flex-1 overflow-y-auto p-4 font-mono text-xs leading-relaxed bg-slate-950 text-slate-300 select-text"
        >
          {filteredLogs.length === 0 ? (
            <div className="text-slate-600 italic py-8 text-center">
              {logs.length === 0 ? 'Awaiting build log events...' : 'No logs match filter.'}
            </div>
          ) : (
            filteredLogs.map((line, idx) => {
              const isError = line.toLowerCase().includes('error') || line.toLowerCase().includes('fail');
              const isSuccess = line.toLowerCase().includes('success') || line.toLowerCase().includes('ready');
              return (
                <div
                  key={idx}
                  className={`flex gap-3 hover:bg-slate-900/60 px-1 py-0.5 rounded ${
                    isError ? 'text-rose-400' : isSuccess ? 'text-emerald-400' : ''
                  }`}
                >
                  <span className="text-slate-600 select-none text-right w-8 shrink-0">{idx + 1}</span>
                  <span className="break-all whitespace-pre-wrap">{line}</span>
                </div>
              );
            })
          )}
        </div>
      </div>
    </div>
  );
};
