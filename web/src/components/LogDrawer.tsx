import React, { useEffect, useRef, useState } from 'react';
import { X, Copy, Check, ArrowDown, Search, Terminal, Play, Command } from 'lucide-react';
import { Deployment } from '../types';

interface LogDrawerProps {
  deployment: Deployment | null;
  initialTab?: 'logs' | 'exec';
  onClose: () => void;
}

export const LogDrawer: React.FC<LogDrawerProps> = ({ deployment, initialTab = 'logs', onClose }) => {
  const [tab, setTab] = useState<'logs' | 'exec'>(initialTab);
  const [logs, setLogs] = useState<string[]>([]);
  const [isConnected, setIsConnected] = useState(false);
  const [autoScroll, setAutoScroll] = useState(true);
  const [copied, setCopied] = useState(false);
  const [filter, setFilter] = useState('');
  const logContainerRef = useRef<HTMLDivElement>(null);

  // Exec state
  const [execCommand, setExecCommand] = useState('ls -la');
  const [execService, setExecService] = useState('web');
  const [execRunning, setExecRunning] = useState(false);
  const [execHistory, setExecHistory] = useState<Array<{ command: string; output: string; time: string; error?: boolean }>>([]);

  useEffect(() => {
    if (initialTab) {
      setTab(initialTab);
    }
  }, [initialTab]);

  useEffect(() => {
    if (!deployment) {
      setLogs([]);
      setExecHistory([]);
      return;
    }

    setExecService(deployment.service_name || 'web');
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
    if (autoScroll && logContainerRef.current && tab === 'logs') {
      logContainerRef.current.scrollTop = logContainerRef.current.scrollHeight;
    }
  }, [logs, autoScroll, tab]);

  const copyLogs = () => {
    navigator.clipboard.writeText(logs.join('\n'));
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const handleRunExec = async (cmdToRun?: string) => {
    const rawCmd = cmdToRun || execCommand;
    if (!rawCmd.trim() || !deployment) return;

    setExecRunning(true);
    const cmdArray = rawCmd.trim().split(/\s+/);

    try {
      const res = await fetch(`/api/deployments/${deployment.id}/exec`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          service: execService,
          command: cmdArray,
        }),
      });

      const data = await res.json();
      const output = data.output || data.error || 'Command exited with no output.';
      const isError = !res.ok;

      setExecHistory((prev) => [
        {
          command: rawCmd,
          output,
          time: new Date().toLocaleTimeString(),
          error: isError,
        },
        ...prev,
      ]);
    } catch (err: any) {
      setExecHistory((prev) => [
        {
          command: rawCmd,
          output: err.message || 'Execution failed',
          time: new Date().toLocaleTimeString(),
          error: true,
        },
        ...prev,
      ]);
    } finally {
      setExecRunning(false);
    }
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
          <div className="flex items-center gap-3">
            <div className="flex items-center gap-1 bg-slate-950 p-1 rounded-lg border border-slate-800">
              <button
                onClick={() => setTab('logs')}
                className={`flex items-center gap-1.5 px-3 py-1 rounded-md text-xs font-medium transition-colors ${
                  tab === 'logs'
                    ? 'bg-slate-800 text-white shadow-sm'
                    : 'text-slate-400 hover:text-slate-200'
                }`}
              >
                <Terminal className="w-3.5 h-3.5 text-emerald-400" />
                <span>Build Logs</span>
              </button>
              <button
                onClick={() => setTab('exec')}
                className={`flex items-center gap-1.5 px-3 py-1 rounded-md text-xs font-medium transition-colors ${
                  tab === 'exec'
                    ? 'bg-slate-800 text-white shadow-sm'
                    : 'text-slate-400 hover:text-slate-200'
                }`}
              >
                <Command className="w-3.5 h-3.5 text-sky-400" />
                <span>Container Exec</span>
              </button>
            </div>

            {tab === 'logs' && (
              <span
                className={`hidden sm:inline-flex items-center gap-1 px-2 py-0.5 rounded text-[10px] font-medium ${
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
            )}
          </div>

          <div className="flex items-center gap-2">
            {tab === 'logs' && (
              <button
                onClick={copyLogs}
                className="flex items-center gap-1 px-2.5 py-1.5 rounded-md bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-medium border border-slate-700 transition-colors"
              >
                {copied ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
                <span>{copied ? 'Copied' : 'Copy'}</span>
              </button>
            )}
            <button
              onClick={onClose}
              className="p-1.5 rounded-md hover:bg-slate-800 text-slate-400 hover:text-white transition-colors"
            >
              <X className="w-5 h-5" />
            </button>
          </div>
        </div>

        {/* Tab 1: Build Logs */}
        {tab === 'logs' && (
          <>
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
          </>
        )}

        {/* Tab 2: Container Exec Console */}
        {tab === 'exec' && (
          <div className="flex-1 flex flex-col overflow-hidden bg-slate-950">
            {/* Quick shortcuts */}
            <div className="px-4 py-2.5 border-b border-slate-800/80 bg-slate-900/40 flex flex-wrap items-center gap-2 text-xs">
              <span className="text-[11px] text-slate-500">Quick run:</span>
              <button
                onClick={() => {
                  setExecCommand('ls -la');
                  handleRunExec('ls -la');
                }}
                className="px-2 py-0.5 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700 text-[11px]"
              >
                ls -la
              </button>
              <button
                onClick={() => {
                  setExecCommand('env');
                  handleRunExec('env');
                }}
                className="px-2 py-0.5 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700 text-[11px]"
              >
                env
              </button>
              <button
                onClick={() => {
                  setExecCommand('df -h');
                  handleRunExec('df -h');
                }}
                className="px-2 py-0.5 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700 text-[11px]"
              >
                df -h
              </button>
              <button
                onClick={() => {
                  setExecCommand('npm run db:migrate');
                  handleRunExec('npm run db:migrate');
                }}
                className="px-2 py-0.5 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700 text-[11px]"
              >
                npm run db:migrate
              </button>
            </div>

            {/* Command Input Form */}
            <form
              onSubmit={(e) => {
                e.preventDefault();
                handleRunExec();
              }}
              className="p-4 border-b border-slate-800 flex items-center gap-3"
            >
              <div className="w-28 shrink-0">
                <input
                  type="text"
                  value={execService}
                  onChange={(e) => setExecService(e.target.value)}
                  placeholder="service"
                  title="Target container service name (e.g. web, api)"
                  className="w-full bg-slate-900 border border-slate-800 rounded-lg px-2.5 py-2 text-xs text-slate-200 font-mono focus:outline-none focus:border-sky-500/50"
                />
              </div>

              <div className="flex-1">
                <input
                  type="text"
                  value={execCommand}
                  onChange={(e) => setExecCommand(e.target.value)}
                  placeholder="command to execute (e.g. npm test, ls -la)"
                  className="w-full bg-slate-900 border border-slate-800 rounded-lg px-3 py-2 text-xs text-slate-100 font-mono focus:outline-none focus:border-sky-500/50"
                />
              </div>

              <button
                type="submit"
                disabled={execRunning}
                className="flex items-center gap-1.5 px-4 py-2 rounded-lg bg-sky-600 hover:bg-sky-500 text-white text-xs font-medium transition-colors disabled:opacity-50 shrink-0"
              >
                <Play className="w-3.5 h-3.5" />
                <span>{execRunning ? 'Running...' : 'Run'}</span>
              </button>
            </form>

            {/* History Console */}
            <div className="flex-1 overflow-y-auto p-4 space-y-4 font-mono text-xs">
              {execHistory.length === 0 ? (
                <div className="text-slate-600 italic py-12 text-center">
                  Execute diagnostics directly inside service <code className="text-slate-400">{execService}</code> without SSH access.
                </div>
              ) : (
                execHistory.map((item, idx) => (
                  <div key={idx} className="bg-slate-900/80 border border-slate-800 rounded-lg p-3">
                    <div className="flex items-center justify-between text-[11px] text-slate-500 mb-1.5 pb-1 border-b border-slate-800/60">
                      <div className="flex items-center gap-2">
                        <span className="text-sky-400 font-semibold">$ {item.command}</span>
                      </div>
                      <span>{item.time}</span>
                    </div>
                    <pre
                      className={`whitespace-pre-wrap break-all leading-relaxed ${
                        item.error ? 'text-rose-400' : 'text-slate-300'
                      }`}
                    >
                      {item.output}
                    </pre>
                  </div>
                ))
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  );
};
