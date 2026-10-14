<script lang="ts">
 import { absoluteTime, relativeTime, formatDuration, safeURL, statusLabel, type Workflow } from '$lib/ci';
 import RunHistory from './RunHistory.svelte';
 let { workflow, now }: { workflow: Workflow; now: number } = $props();
 let latest = $derived(workflow.latest_run);
 let latestURL = $derived(latest ? safeURL(latest.url) : null);
 let repoURL = $derived(safeURL(workflow.repository.url));
</script>
<article class="workflow-row" aria-label={`${workflow.repository.full_name}, ${workflow.name}`}>
 <div class="workflow-identity">
  <div class="repository-line"><span class="provider-name">{workflow.repository.source}</span><span aria-hidden="true">/</span>{#if repoURL}<a href={repoURL} target="_blank" rel="noopener noreferrer">{workflow.repository.full_name}</a>{:else}<span>{workflow.repository.full_name}</span>{/if}</div>
  <h2>{#if latestURL}<a href={latestURL} target="_blank" rel="noopener noreferrer">{workflow.name}</a>{:else}{workflow.name}{/if}</h2>
  <div class="workflow-scope">{workflow.branch_scope === 'all' ? 'All branches' : workflow.branch_scope}<span aria-hidden="true"> · </span>{workflow.runs.length} observed{#if workflow.coverage.partial}<span> · Limited history</span>{/if}</div>
  {#if workflow.health.state !== 'healthy' && workflow.health.state !== 'ok'}<p class="row-notice">{workflow.health.state === 'loading' ? 'Loading repository' : 'Stale data'}{workflow.health.message ? ` · ${workflow.health.message}` : ''}{#if workflow.health.last_success_at} · Updated {relativeTime(workflow.health.last_success_at, now)}{/if}{#if workflow.health.retry_at} · Retry {absoluteTime(workflow.health.retry_at)}{/if}</p>{/if}
 </div>
 <div class="workflow-history"><RunHistory runs={workflow.runs}/></div>
 <dl class="workflow-metrics">
  <div title="Execution duration of the newest completed run"><dt>Execution</dt><dd>{formatDuration(workflow.last_completed_duration_ms)}</dd></div>
  <div title="Successful runs divided by successful and failed terminal runs in the configured sample"><dt>Success rate</dt><dd>{workflow.success_rate === null ? '—' : `${Math.round(workflow.success_rate * 100)}%`}<small>{workflow.eligible_runs} eligible</small></dd></div>
  <div><dt>Last run</dt><dd>{#if latest}<time datetime={latest.created_at} title={absoluteTime(latest.created_at)}>{relativeTime(latest.created_at, now)}</time>{:else}—{/if}</dd></div>
 </dl>
 <div class="latest-state" data-state={latest?.state ?? 'unknown'}><span class="status-dot" aria-hidden="true"></span>{#if latestURL}<a href={latestURL} target="_blank" rel="noopener noreferrer" aria-label={`${latest ? statusLabel(latest.state) : 'No runs'}: open latest ${workflow.name} run`}>{latest ? statusLabel(latest.state) : 'No runs'}</a>{:else}<span>{latest ? statusLabel(latest.state) : 'No runs'}</span>{/if}</div>
</article>
