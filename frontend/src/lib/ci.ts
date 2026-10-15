export const states = ['queued', 'waiting', 'running', 'success', 'failure', 'canceled', 'skipped', 'neutral', 'attention', 'unknown'] as const;
export type RunState = typeof states[number];
export type Repository = { key: string; source: string; provider: string; id: string; owner: string; name: string; full_name: string; default_branch: string; url: string };
export type Run = { id: string; workflow_id: string; workflow_name: string; title: string; branch: string; event: string; url: string; attempt: number; state: RunState; terminal: boolean; raw_status: string; raw_conclusion: string; created_at: string; started_at: string | null; finished_at: string | null; duration_ms: number | null; duration_kind: string };
export type Health = { state: string; message: string; last_attempt_at: string; last_success_at: string | null; retry_at: string | null };
export type Coverage = { partial: boolean; fetched_runs: number; oldest_fetched_at: string | null };
export type Workflow = { file_exists?: boolean | null; key: string; repository: Repository; workflow_id: string; workflow_url?: string; name: string; branch_scope: string; runs: Run[]; latest_run: Run | null; last_completed_duration_ms: number | null; duration_kind: string; success_rate: number | null; eligible_runs: number; coverage: Coverage; health: Health };
export type Source = { name: string; provider: string; state: string; repositories: string[]; issues: { repository: string; kind: string; message: string }[]; last_discovery_at: string | null };
export type Overview = { workflows: Workflow[]; sources: Source[]; generated_at: string; poll_interval_seconds: number };
export type Filters = { search: string; provider: string; source: string; repository: string; status: string };

function fail(): never { throw new Error('The server returned an invalid CI overview.'); }
function object(v: unknown): Record<string, unknown> { if (!v || typeof v !== 'object' || Array.isArray(v)) return fail(); return v as Record<string, unknown>; }
function string(v: unknown): string { return typeof v === 'string' ? v : fail(); }
function bool(v: unknown): boolean { return typeof v === 'boolean' ? v : fail(); }
function number(v: unknown): number { return typeof v === 'number' && Number.isFinite(v) && v >= 0 ? v : fail(); }
function integer(v: unknown): number { const n = number(v); return Number.isSafeInteger(n) ? n : fail(); }
function date(v: unknown): string { const s = string(v); if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z$/.test(s) || !Number.isFinite(Date.parse(s)) || new Date(s).toISOString().slice(0, 19) !== s.slice(0, 19)) fail(); return s; }
function nullable<T>(v: unknown, parse: (v: unknown) => T): T | null { return v === null ? null : parse(v); }
function list<T>(v: unknown, parse: (v: unknown) => T): T[] { return Array.isArray(v) ? v.map(parse) : fail(); }
function identity(v: unknown): string { const s = string(v); return s.trim() ? s : fail(); }
function unique<T>(items: T[], key: (item: T) => string): T[] { const keys = items.map(key); if (new Set(keys).size !== keys.length) fail(); return items; }
function repository(v: unknown): Repository {
 const o = object(v); return { key: identity(o.key), source: string(o.source), provider: string(o.provider), id: identity(o.id), owner: string(o.owner), name: string(o.name), full_name: string(o.full_name), default_branch: string(o.default_branch), url: string(o.url) };
}
function run(v: unknown): Run {
 const o = object(v); const state = string(o.state); if (!states.includes(state as RunState)) fail();
 return { id: identity(o.id), workflow_id: string(o.workflow_id), workflow_name: string(o.workflow_name), title: string(o.title), branch: string(o.branch), event: string(o.event), url: string(o.url), attempt: integer(o.attempt), state: state as RunState, terminal: bool(o.terminal), raw_status: string(o.raw_status), raw_conclusion: string(o.raw_conclusion), created_at: date(o.created_at), started_at: nullable(o.started_at, date), finished_at: nullable(o.finished_at, date), duration_ms: nullable(o.duration_ms, integer), duration_kind: string(o.duration_kind) };
}
function workflow(v: unknown): Workflow {
 const o = object(v); const h = object(o.health); const c = object(o.coverage);
 const rate = nullable(o.success_rate, number); if (rate !== null && rate > 1) fail();
 return { file_exists: o.file_exists === undefined ? null : nullable(o.file_exists, bool), key: identity(o.key), repository: repository(o.repository), workflow_id: string(o.workflow_id), workflow_url: o.workflow_url === undefined ? '' : string(o.workflow_url), name: string(o.name), branch_scope: string(o.branch_scope), runs: unique(list(o.runs, run), item => item.id), latest_run: nullable(o.latest_run, run), last_completed_duration_ms: nullable(o.last_completed_duration_ms, integer), duration_kind: string(o.duration_kind), success_rate: rate, eligible_runs: integer(o.eligible_runs), coverage: { partial: bool(c.partial), fetched_runs: integer(c.fetched_runs), oldest_fetched_at: nullable(c.oldest_fetched_at, date) }, health: { state: string(h.state), message: string(h.message), last_attempt_at: date(h.last_attempt_at), last_success_at: nullable(h.last_success_at, date), retry_at: nullable(h.retry_at, date) } };
}
export function parseOverview(v: unknown): Overview {
 const o = object(v); const interval = integer(o.poll_interval_seconds); if (interval < 1) fail();
 return { workflows: unique(list(o.workflows, workflow), item => item.key), sources: unique(list(o.sources, v => { const s = object(v); return { name: identity(s.name), provider: string(s.provider), state: string(s.state), repositories: list(s.repositories, string), issues: list(s.issues, v => { const i = object(v); return { repository: string(i.repository), kind: string(i.kind), message: string(i.message) }; }), last_discovery_at: nullable(s.last_discovery_at, date) }; }), item => item.name), generated_at: date(o.generated_at), poll_interval_seconds: interval };
}
export function safeURL(value: string): string | null {
 if (!/^https?:\/\//i.test(value) || /[\s\\\u0000-\u001f\u007f]/.test(value)) return null;
 try { const url = new URL(value); return ['https:', 'http:'].includes(url.protocol) && !url.username && !url.password ? value : null; } catch { return null; }
}
export function filterWorkflows(rows: Workflow[], filters: Filters): Workflow[] {
 const words = filters.search.trim().toLocaleLowerCase().split(/\s+/).filter(Boolean);
 return rows.filter(row => {
  const haystack = [row.name, row.workflow_id, row.repository.full_name, row.repository.source, row.repository.provider, row.branch_scope].join(' ').toLocaleLowerCase();
  return words.every(word => haystack.includes(word)) && (!filters.provider || filters.provider === row.repository.provider) && (!filters.source || filters.source === row.repository.source) && (!filters.repository || filters.repository === row.repository.key) && (!filters.status || filters.status === row.latest_run?.state);
 }).sort((a, b) => {
  for (const [left, right] of [[a.repository.source, b.repository.source], [a.repository.full_name, b.repository.full_name], [a.name, b.name], [a.key, b.key]]) { const order = left.localeCompare(right); if (order) return order; }
  return 0;
 });
}
export type RepositoryGroup = { repository: Repository; workflows: Workflow[] };
export type SourceGroup = { name: string; repositories: RepositoryGroup[] };
export function groupWorkflows(rows: Workflow[]): SourceGroup[] {
 const sources = new Map<string, Map<string, RepositoryGroup>>();
 for (const row of rows) {
  let repositories = sources.get(row.repository.source);
  if (!repositories) { repositories = new Map(); sources.set(row.repository.source, repositories); }
  let group = repositories.get(row.repository.key);
  if (!group) { group = { repository: row.repository, workflows: [] }; repositories.set(row.repository.key, group); }
  group.workflows.push(row);
 }
 return [...sources].sort(([a], [b]) => a.localeCompare(b)).map(([name, repositories]) => ({
  name,
  repositories: [...repositories.values()].sort((a, b) => a.repository.full_name.localeCompare(b.repository.full_name) || a.repository.key.localeCompare(b.repository.key)).map(group => ({
   repository: group.repository,
   workflows: group.workflows.sort((a, b) => a.name.localeCompare(b.name) || a.key.localeCompare(b.key)),
  })),
 }));
}

export function formatDuration(ms: number | null): string {
 if (ms === null) return '—'; const seconds = Math.floor(ms / 1000);
 if (seconds >= 3600) return `${Math.floor(seconds / 3600)}h ${Math.floor(seconds % 3600 / 60)}m`;
 if (seconds >= 60) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
 return `${seconds}s`;
}
export function sampleSuccess(runs: Run[]): { rate: number | null; eligible: number } {
 const eligible = runs.filter(r => r.terminal && (r.state === 'success' || r.state === 'failure'));
 return { rate: eligible.length ? eligible.filter(r => r.state === 'success').length / eligible.length : null, eligible: eligible.length };
}
export function readFilters(params: URLSearchParams): Filters {
 const status = params.get('status') ?? ''; return { search: params.get('q') ?? '', provider: params.get('provider') ?? '', source: params.get('source') ?? '', repository: params.get('repo') ?? '', status: states.includes(status as RunState) ? status : '' };
}
export function writeFilters(filters: Filters, previous = new URLSearchParams()): URLSearchParams {
 const params = new URLSearchParams(previous); for (const [key, value] of Object.entries({ q: filters.search, provider: filters.provider, source: filters.source, repo: filters.repository, status: filters.status })) { if (value) params.set(key, value); else params.delete(key); } return params;
}
export function statusLabel(state: RunState): string { return ({ queued: 'Queued', waiting: 'Waiting', running: 'Running', success: 'Passed', failure: 'Failed', canceled: 'Canceled', skipped: 'Skipped', neutral: 'Neutral', attention: 'Needs attention', unknown: 'Unknown' })[state]; }
export function absoluteTime(value: string | null): string { return value === null ? 'Unavailable' : new Date(value).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' }); }
export function relativeTime(value: string, now = Date.now()): string {
 const seconds = Math.max(0, Math.floor((now - Date.parse(value)) / 1000));
 if (seconds < 60) return 'Just now'; const rtf = new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' });
 if (seconds < 3600) return rtf.format(-Math.floor(seconds / 60), 'minute');
 if (seconds < 86400) return rtf.format(-Math.floor(seconds / 3600), 'hour');
 return rtf.format(-Math.floor(seconds / 86400), 'day');
}
export function runLabel(run: Run): string { return `${run.title || `Run ${run.id}`} · ${run.branch} · ${statusLabel(run.state)} · ${absoluteTime(run.created_at)}${run.duration_ms === null ? '' : ` · Execution ${formatDuration(run.duration_ms)}`}`; }

// The overview API already orders each displayed sample oldest to newest.
export function historyRuns(runs: Run[]): Run[] { return [...runs]; }

export function sourceIssueTone(kind: string): 'info' | 'loading' | 'attention' {
 if (kind === 'loading') return 'loading';
 return ['no_observed_runs', 'limited_history'].includes(kind) ? 'info' : 'attention';
}

export function sourceNotices(sources: Source[]): Source[] {
 return sources.map(source => ({ ...source, issues: source.issues.filter(issue => issue.kind !== 'no_observed_runs') }))
  .filter(source => source.issues.length > 0 || source.state !== 'healthy' && source.state !== 'ok');
}
