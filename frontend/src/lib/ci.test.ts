import { test } from 'node:test';
import assert from 'node:assert/strict';
import type { Run, RunState, Workflow } from './ci.ts';
import { parseOverview, historyRuns, sourceIssueTone, safeURL, filterWorkflows, formatDuration, sampleSuccess, readFilters, writeFilters } from './ci.ts';

const repository = { key: 'repo-1', source: 'github', provider: 'github', id: '1', owner: 'example', name: 'api', full_name: 'example/api', default_branch: 'main', url: 'https://github.com/example/api' };
const run: Run = { id: '101', workflow_id: '10', workflow_name: 'Tests', title: 'Build', branch: 'main', event: 'push', url: 'https://github.com/example/api/actions/runs/101', attempt: 1, state: 'success', terminal: true, raw_status: 'completed', raw_conclusion: 'success', created_at: '2026-10-14T12:00:00Z', started_at: null, finished_at: null, duration_ms: null, duration_kind: '' };
const workflow: Workflow = { key: 'row-1', repository, workflow_id: '10', name: 'Tests', branch_scope: 'main', runs: [run], latest_run: run, last_completed_duration_ms: null, duration_kind: '', success_rate: 1, eligible_runs: 1, coverage: { partial: false, fetched_runs: 1, oldest_fetched_at: null }, health: { state: 'healthy', message: '', last_attempt_at: '2026-10-14T12:00:00Z', last_success_at: null, retry_at: null } };
const overview = () => ({ workflows: [structuredClone(workflow)], sources: [{ name: 'github', provider: 'github', state: 'healthy', repositories: ['repo-1'], issues: [], last_discovery_at: null }], generated_at: '2026-10-14T12:00:00Z', poll_interval_seconds: 60 });

test('parses the exact contract with nullable and zero metrics', () => {
 const data = overview(); data.workflows[0].last_completed_duration_ms = 0;
 const parsed = parseOverview(data); assert.equal(parsed.workflows[0].last_completed_duration_ms, 0);
 assert.equal(parsed.workflows[0].runs[0].duration_ms, null);
});
test('rejects malformed JSON shapes and invalid values', () => {
 for (const bad of [null, [], {}, { ...overview(), workflows: null }, { ...overview(), poll_interval_seconds: 0 }, { ...overview(), generated_at: 'yesterday' }]) assert.throws(() => parseOverview(bad));
 const bad = overview(); Object.assign(bad.workflows[0].runs[0], { state: 'great' }); assert.throws(() => parseOverview(bad));
 const negative = overview(); negative.workflows[0].eligible_runs = -1; assert.throws(() => parseOverview(negative));
 const missing = overview(); Reflect.deleteProperty(missing.workflows[0].repository, 'key'); assert.throws(() => parseOverview(missing));
});
test('only absolute HTTP(S) links without embedded credentials are clickable', () => {
 assert.equal(safeURL(run.url), run.url);
 assert.equal(safeURL('https://forge.example.net/prefix/actions/runs/1?attempt=2'), 'https://forge.example.net/prefix/actions/runs/1?attempt=2');
 for (const url of ['javascript:alert(1)', 'data:text/html,x', '//evil.test', '/runs/1', 'https://name:secret@evil.test', ' https://example.net', 'https://example.net\n']) assert.equal(safeURL(url), null);
});
test('combines text, provider, source, repository and latest status filters', () => {
 const second: Workflow = { ...workflow, key: 'row-2', name: 'Release', repository: { ...repository, key: 'repo-2', source: 'forge', provider: 'forgejo' }, latest_run: { ...run, state: 'failure' } };
 const rows = [workflow, second];
 assert.deepEqual(filterWorkflows(rows, { search: 'EXAMPLE test', provider: 'github', source: 'github', repository: 'repo-1', status: 'success' }).map(r => r.key), ['row-1']);
 assert.equal(filterWorkflows(rows, { search: '', provider: 'forgejo', source: '', repository: '', status: 'failure' }).length, 1);
 assert.equal(filterWorkflows(rows, { search: '', provider: '', source: '', repository: 'gone', status: '' }).length, 0);
});
test('preserves unavailable and zero execution duration', () => {
 assert.equal(formatDuration(null), '—'); assert.equal(formatDuration(0), '0s');
 assert.equal(formatDuration(65000), '1m 5s'); assert.equal(formatDuration(3600000), '1h 0m');
});
test('sample rate excludes unfinished, canceled, skipped, neutral, attention and unknown', () => {
 const runs = (['success', 'failure', 'canceled', 'skipped', 'neutral', 'attention', 'unknown', 'running'] as RunState[]).map(state => ({ ...run, state, terminal: state !== 'running' }));
 assert.deepEqual(sampleSuccess(runs), { rate: 0.5, eligible: 2 });
 assert.deepEqual(sampleSuccess([{ ...run, state: 'unknown' }]), { rate: null, eligible: 0 });
});
test('URL filters remain unchanged when a refresh removes selected options', () => {
 const params = new URLSearchParams('repo=removed&q=api&status=failure&other=keep');
 const filters = readFilters(params); assert.equal(filters.repository, 'removed');
 const next = writeFilters(filters, params); assert.equal(next.get('repo'), 'removed'); assert.equal(next.get('other'), 'keep');
 assert.equal(readFilters(new URLSearchParams('status=bogus')).status, '');
});

test('rejects duplicate render identities, fractional duration, invalid UTC dates and metrics', () => {
 const duplicate = overview(); duplicate.workflows.push(structuredClone(workflow)); assert.throws(() => parseOverview(duplicate));
 const duplicateRuns = overview(); duplicateRuns.workflows[0].runs.push(structuredClone(run)); assert.throws(() => parseOverview(duplicateRuns));
 const duplicateSources = overview(); duplicateSources.sources.push(structuredClone(duplicateSources.sources[0])); assert.throws(() => parseOverview(duplicateSources));
 const badDate = overview(); badDate.generated_at = '2026-02-31T12:00:00Z'; assert.throws(() => parseOverview(badDate));
 const badRate = overview(); badRate.workflows[0].success_rate = 1.1; assert.throws(() => parseOverview(badRate));
 const fractional = overview(); fractional.workflows[0].last_completed_duration_ms = 1.5; assert.throws(() => parseOverview(fractional));
});
test('filter sorting is stable and leaves source snapshots unchanged', () => {
 const second: Workflow = { ...structuredClone(workflow), key: 'row-0', name: 'A check' };
 const rows = [workflow, second]; const filter = readFilters(new URLSearchParams());
 assert.deepEqual(filterWorkflows(rows, filter).map(row => row.key), ['row-0', 'row-1']);
 assert.equal(rows[0].key, 'row-1');
});

test('history preserves API chronology so the rightmost bar is the newest run', () => {
 const oldest: Run = { ...run, id: 'oldest', created_at: '2026-10-13T12:00:00Z' };
 const newest: Run = { ...run, id: 'newest', created_at: '2026-10-14T12:00:00Z', state: 'running', terminal: false };
 const sample = [oldest, newest];
 const bars = historyRuns(sample);
 assert.equal(bars[0].id, 'oldest');
 assert.equal(bars.at(-1)?.id, 'newest');
 assert.deepEqual(sample.map(item => item.id), ['oldest', 'newest']);
});

test('source issues distinguish history information, loading and unavailable selections', () => {
 assert.equal(sourceIssueTone('no_observed_runs'), 'info');
 assert.equal(sourceIssueTone('limited_history'), 'info');
 assert.equal(sourceIssueTone('loading'), 'loading');
 assert.equal(sourceIssueTone('permission'), 'attention');
 assert.equal(sourceIssueTone('authentication'), 'attention');
});
