<script lang="ts">
 import { absoluteTime, sourceIssueTone, type Source } from '$lib/ci';
 let { sources }: { sources: Source[] } = $props();
</script>
{#each sources as source (source.name)}
 {#if source.state !== 'healthy' && source.state !== 'ok' || source.issues.length}
  <section class="source-notice" data-tone={source.state === 'healthy' || source.state === 'ok' ? 'info' : 'attention'} aria-label={`${source.name} source status`}>
   <p><strong>{source.name}</strong> · {source.state === 'loading' ? 'Loading repositories' : source.state === 'healthy' || source.state === 'ok' ? 'Source notices' : 'Source needs attention'}</p>
   {#each source.issues as issue, i (i)}<p class="source-issue" data-tone={sourceIssueTone(issue.kind)}><span class="issue-label">{sourceIssueTone(issue.kind) === 'info' ? 'History' : sourceIssueTone(issue.kind) === 'loading' ? 'Loading' : 'Attention'}</span>{issue.repository ? `${issue.repository}: ` : ''}{issue.message || issue.kind}</p>{/each}
   {#if source.last_discovery_at}<small>Last discovery {absoluteTime(source.last_discovery_at)}</small>{/if}
  </section>
 {/if}
{/each}
