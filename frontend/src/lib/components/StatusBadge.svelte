<script lang="ts" module>
  export type BadgeTone = "success" | "danger" | "info" | "warning" | "neutral";
</script>

<script lang="ts">
  import type { Snippet } from "svelte";

  interface Props {
    tone?: BadgeTone;
    dot?: boolean;
    title?: string;
    children?: Snippet;
  }
  let { tone = "neutral", dot = false, title, children }: Props = $props();
</script>

<span data-slot="badge" class="status-badge" data-tone={tone} {title}>
  {#if dot}<i class="status-dot" aria-hidden="true"></i>{/if}{@render children?.()}
</span>
