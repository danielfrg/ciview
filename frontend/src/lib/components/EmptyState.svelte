<script lang="ts">
  import type { Snippet } from "svelte";

  interface Props {
    title: string;
    description?: string;
    children?: Snippet;
  }
  let { title, description = "", children }: Props = $props();
</script>

<div class="empty-state">
  <h2>{title}</h2>
  {#if description}<p>{description}</p>{/if}
  {#if children}<div class="empty-state-action">{@render children()}</div>{/if}
</div>
