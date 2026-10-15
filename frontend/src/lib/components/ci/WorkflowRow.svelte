<script lang="ts">
 import { absoluteTime, relativeTime, formatDuration, safeURL, statusLabel, type Workflow } from '$lib/ci';
 import { Chip, type ChipTone } from '@kenn-io/kit-ui';
 import RunHistory from './RunHistory.svelte';
 let { workflow, now }: { workflow: Workflow; now: number } = $props();
 let latest = $derived(workflow.latest_run);
 const tones: Record<string, ChipTone> = { success: 'success', failure: 'danger', running: 'info', queued: 'warning', waiting: 'warning', attention: 'warning', canceled: 'neutral', skipped: 'neutral', neutral: 'neutral', unknown: 'neutral' };
 let latestURL = $derived(latest ? safeURL(latest.url) : null);
 let repositoryURL = $derived(safeURL(workflow.repository.url));
 let workflowURL = $derived(safeURL(workflow.workflow_url ?? ''));
</script>
<article class="workflow-row" aria-label={`${workflow.repository.full_name}, ${workflow.name}`}>
 <div class="workflow-identity">
  <h3 class="workflow-title">{#if repositoryURL}<a href={repositoryURL} target="_blank" rel="noopener noreferrer">{workflow.repository.full_name}</a>{:else}{workflow.repository.full_name}{/if} - {#if workflowURL}<a href={workflowURL} target="_blank" rel="noopener noreferrer" title="Open workflow history">{workflow.name}</a>{:else}{workflow.name}{/if}</h3>
  <div class="workflow-meta"><span class="workflow-scope">{workflow.branch_scope === 'all' ? 'All branches' : workflow.branch_scope}</span>{#if workflow.file_exists === false}<Chip tone="warning" size="sm" uppercase={false} title={`Workflow file is no longer present on ${workflow.branch_scope}. Historical runs are retained.`}>Not on branch</Chip>{/if}</div>
  {#if workflow.health.state !== 'healthy' && workflow.health.state !== 'ok'}<p class="row-notice">{workflow.health.state === 'loading' ? 'Loading repository' : 'Stale data'}{workflow.health.message ? ` · ${workflow.health.message}` : ''}{#if workflow.health.last_success_at} · Updated {relativeTime(workflow.health.last_success_at, now)}{/if}{#if workflow.health.retry_at} · Retry {absoluteTime(workflow.health.retry_at)}{/if}</p>{/if}
 </div>
 <div class="workflow-history"><RunHistory runs={workflow.runs}/></div>
 <dl class="workflow-metrics">
  <div title="Execution duration of the newest completed run"><dt>Execution</dt><dd>{formatDuration(workflow.last_completed_duration_ms)}</dd></div>
  <div title="Successful runs divided by successful and failed terminal runs in the configured sample"><dt>Success rate</dt><dd>{workflow.success_rate === null ? '—' : `${Math.round(workflow.success_rate * 100)}%`}</dd></div>
  <div><dt>Last run</dt><dd>{#if latest}<time datetime={latest.created_at} title={absoluteTime(latest.created_at)}>{relativeTime(latest.created_at, now)}</time>{:else}—{/if}</dd></div>
 </dl>
 <div class="latest-state" data-state={latest?.state ?? 'unknown'}>{#if latestURL}<a href={latestURL} target="_blank" rel="noopener noreferrer" aria-label={`${latest ? statusLabel(latest.state) : 'No runs'}: open latest ${workflow.name} run`}><Chip tone={tones[latest?.state ?? 'unknown']} size="sm" dot uppercase={false}>{latest ? statusLabel(latest.state) : 'No runs'}</Chip></a>{:else}<Chip tone={tones[latest?.state ?? 'unknown']} size="sm" dot uppercase={false}>{latest ? statusLabel(latest.state) : 'No runs'}</Chip>{/if}</div>
</article>
