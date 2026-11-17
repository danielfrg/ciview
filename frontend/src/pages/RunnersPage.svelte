<script lang="ts">
  import EmptyState from "$lib/components/EmptyState.svelte";
  import RunnerPanel from "$lib/components/ci/RunnerPanel.svelte";
  import type { RunnerScope } from "$lib/ci";
  let { scopes, now, loading }: { scopes: RunnerScope[]; now: number; loading: boolean } = $props();
</script>

{#if scopes.length}
  <RunnerPanel {scopes} {now} />
{:else if !loading}
  <div class="workflow-state empty-state">
    <EmptyState
      title="No runners configured"
      description="Add a [[runners]] scope to the configuration to watch self-hosted runners."
    />
  </div>
{/if}
