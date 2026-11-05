import React, { useState } from 'react';
import { X, Copy, Check, ShieldCheck, Globe, Key, Sliders, ExternalLink, Moon, Bell, Lock } from 'lucide-react';
import { SystemConfig } from '../types';

interface SettingsModalProps {
  isOpen: boolean;
  config: SystemConfig | null;
  onClose: () => void;
}

export const SettingsModal: React.FC<SettingsModalProps> = ({ isOpen, config, onClose }) => {
  const [copiedWebhook, setCopiedWebhook] = useState(false);

  if (!isOpen || !config) return null;

  const webhookURL = `${window.location.origin}/webhook`;

  const copyWebhook = () => {
    navigator.clipboard.writeText(webhookURL);
    setCopiedWebhook(true);
    setTimeout(() => setCopiedWebhook(false), 2000);
  };

  return (
    <div className="fixed inset-0 z-50 overflow-y-auto bg-slate-950/70 backdrop-blur-sm flex items-center justify-center p-4">
      <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-xl shadow-2xl p-6">
        <div className="flex items-center justify-between mb-4 pb-3 border-b border-slate-800">
          <div className="flex items-center gap-2">
            <div className="p-2 rounded-lg bg-emerald-500/10 text-emerald-400">
              <Sliders className="w-4 h-4" />
            </div>
            <div>
              <h2 className="text-base font-semibold text-slate-100">System Configuration</h2>
              <p className="text-xs text-slate-400">Supervisor runtime settings, security and integrations</p>
            </div>
          </div>
          <button onClick={onClose} className="p-1 rounded-md text-slate-400 hover:text-white">
            <X className="w-5 h-5" />
          </button>
        </div>

        <div className="space-y-4">
          {/* GitHub Webhook Setup Guide */}
          <div className="bg-slate-950/50 border border-slate-800 rounded-xl p-4">
            <div className="flex items-center justify-between mb-2">
              <span className="text-xs font-semibold text-slate-200">GitHub Webhook Setup</span>
              <a
                href="https://docs.github.com/en/webhooks/using-webhooks/creating-webhooks"
                target="_blank"
                rel="noreferrer"
                className="text-[11px] text-emerald-400 hover:underline flex items-center gap-1"
              >
                Docs <ExternalLink className="w-3 h-3" />
              </a>
            </div>
            <p className="text-xs text-slate-400 mb-3">
              Add this payload URL in your GitHub repository settings under <strong>Webhooks &gt; Add webhook</strong>:
            </p>

            <div className="flex items-center gap-2 mb-3">
              <input
                type="text"
                readOnly
                value={webhookURL}
                className="w-full bg-slate-900 border border-slate-800 rounded px-3 py-1.5 text-xs font-mono text-slate-200 select-all focus:outline-none"
              />
              <button
                onClick={copyWebhook}
                className="px-3 py-1.5 rounded bg-slate-800 hover:bg-slate-700 text-xs font-medium text-slate-300 border border-slate-700 flex items-center gap-1 shrink-0"
              >
                {copiedWebhook ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
                <span>{copiedWebhook ? 'Copied' : 'Copy'}</span>
              </button>
            </div>

            <div className="grid grid-cols-2 gap-2 text-[11px] text-slate-400 bg-slate-900/60 p-2.5 rounded border border-slate-800/80">
              <div>Content type: <strong className="text-slate-200">application/json</strong></div>
              <div>Events: <strong className="text-slate-200">Pull requests</strong></div>
            </div>
          </div>

          {/* Configuration Matrix */}
          <div className="grid grid-cols-2 gap-3">
            <div className="bg-slate-950/50 border border-slate-800 rounded-lg p-3">
              <div className="flex items-center gap-1.5 text-xs text-slate-400 mb-1">
                <Globe className="w-3.5 h-3.5 text-slate-500" />
                <span>Base Subdomain</span>
              </div>
              <div className="text-xs font-mono font-medium text-slate-200 break-all">
                *.{config.base_domain}
              </div>
            </div>

            <div className="bg-slate-950/50 border border-slate-800 rounded-lg p-3">
              <div className="flex items-center gap-1.5 text-xs text-slate-400 mb-1">
                <Moon className="w-3.5 h-3.5 text-slate-500" />
                <span>Auto-Sleep Hibernation</span>
              </div>
              <div className="text-xs font-medium text-slate-200">
                {config.auto_sleep_minutes > 0 ? (
                  <span className="text-emerald-400">Active ({config.auto_sleep_minutes}m idle)</span>
                ) : (
                  <span className="text-slate-400">Disabled</span>
                )}
              </div>
            </div>

            <div className="bg-slate-950/50 border border-slate-800 rounded-lg p-3">
              <div className="flex items-center gap-1.5 text-xs text-slate-400 mb-1">
                <ShieldCheck className="w-3.5 h-3.5 text-slate-500" />
                <span>HMAC Verification</span>
              </div>
              <div className="text-xs font-medium">
                {config.has_webhook_secret ? (
                  <span className="text-emerald-400">Active (Secret Configured)</span>
                ) : (
                  <span className="text-amber-400">Open (No secret set)</span>
                )}
              </div>
            </div>

            <div className="bg-slate-950/50 border border-slate-800 rounded-lg p-3">
              <div className="flex items-center gap-1.5 text-xs text-slate-400 mb-1">
                <Key className="w-3.5 h-3.5 text-slate-500" />
                <span>GitHub Bot Comments</span>
              </div>
              <div className="text-xs font-medium">
                {config.has_github_token ? (
                  <span className="text-emerald-400">Connected</span>
                ) : (
                  <span className="text-slate-400">Read-Only (No token)</span>
                )}
              </div>
            </div>

            <div className="bg-slate-950/50 border border-slate-800 rounded-lg p-3">
              <div className="flex items-center gap-1.5 text-xs text-slate-400 mb-1">
                <Lock className="w-3.5 h-3.5 text-slate-500" />
                <span>Password Protection</span>
              </div>
              <div className="text-xs font-medium">
                {config.has_basic_auth ? (
                  <span className="text-emerald-400">Basic Auth Active</span>
                ) : (
                  <span className="text-slate-400">Public (No password)</span>
                )}
              </div>
            </div>

            <div className="bg-slate-950/50 border border-slate-800 rounded-lg p-3">
              <div className="flex items-center gap-1.5 text-xs text-slate-400 mb-1">
                <Bell className="w-3.5 h-3.5 text-slate-500" />
                <span>Team Chat Webhooks</span>
              </div>
              <div className="text-xs font-medium">
                {config.has_discord_webhook || config.has_slack_webhook ? (
                  <span className="text-emerald-400">Connected</span>
                ) : (
                  <span className="text-slate-400">Not configured</span>
                )}
              </div>
            </div>
          </div>

          {/* Port Range & Timeout */}
          <div className="bg-slate-950/50 border border-slate-800 rounded-lg p-3 flex items-center justify-between text-xs">
            <span className="text-slate-400">Host Port Allocation Range</span>
            <span className="font-mono text-slate-200">
              {config.port_range_start} - {config.port_range_end}
            </span>
          </div>

          <div className="bg-slate-950/50 border border-slate-800 rounded-lg p-3 flex items-center justify-between text-xs">
            <span className="text-slate-400">Inactive PR Reaper TTL</span>
            <span className="text-slate-200">{config.idle_timeout_minutes / 60} hours</span>
          </div>
        </div>

        <div className="mt-6 flex justify-end">
          <button
            onClick={onClose}
            className="px-4 py-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-medium transition-colors"
          >
            Close
          </button>
        </div>
      </div>
    </div>
  );
};
