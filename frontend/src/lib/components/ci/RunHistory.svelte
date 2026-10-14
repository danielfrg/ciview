<script lang="ts">
 import { safeURL, runLabel, historyRuns, type Run } from '$lib/ci';
 let { runs }: { runs: Run[] } = $props();
 let recent = $derived(historyRuns(runs));
 let tooltip = $state<{ label: string; left: number; top: number } | null>(null);
 function show(event: Event, run: Run) { const rect = (event.currentTarget as HTMLElement).getBoundingClientRect(); tooltip = { label: `${runLabel(run)}${safeURL(run.url) ? '' : ' · Link unavailable'}`, left: Math.max(8, Math.min(rect.left - 120, window.innerWidth - 268)), top: Math.min(rect.bottom + 6, window.innerHeight - 130) }; }
</script>
<!-- svelte-ignore a11y_no_noninteractive_tabindex (scrollable history must be keyboard accessible) -->
<div class="history-scroll" tabindex={runs.length > 30 ? 0 : undefined} role="group" aria-label="Observed run history, oldest to newest">
 <div class="run-history" style={`--run-count: ${Math.max(30, recent.length)}; min-width: ${Math.max(250, recent.length * 9 - 3)}px`}>
  {#each Array(Math.max(0, 30 - recent.length)) as _, i (i)}<span class="run-slot" aria-hidden="true"></span>{/each}
  {#each recent as run (run.id)}
   {@const url = safeURL(run.url)}
   <div class="run-item">
    {#if url}<a class="run-bar" data-state={run.state} href={url} target="_blank" rel="noopener noreferrer" aria-label={runLabel(run)} onmouseenter={event => show(event, run)} onmouseleave={() => tooltip = null} onfocus={event => show(event, run)} onblur={() => tooltip = null} onkeydown={event => { if (event.key === "Escape") tooltip = null; }}><span class="sr-only">{runLabel(run)}</span></a>
    {:else}<button type="button" class="run-bar" data-state={run.state} aria-disabled="true" aria-label={`${runLabel(run)} · Link unavailable`} onmouseenter={event => show(event, run)} onmouseleave={() => tooltip = null} onfocus={event => show(event, run)} onblur={() => tooltip = null} onkeydown={event => { if (event.key === "Escape") tooltip = null; }} onclick={event => show(event, run)}></button>{/if}
   </div>
  {/each}
 </div>
</div>
{#if tooltip}<span class="run-tooltip" style={`left: ${tooltip.left}px; top: ${tooltip.top}px`} aria-hidden="true">{tooltip.label}</span>{/if}
