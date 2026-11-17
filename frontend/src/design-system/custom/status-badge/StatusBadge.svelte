<script lang="ts">
  import type { Snippet } from "svelte";
  import type { HTMLAttributes } from "svelte/elements";

  interface Props extends HTMLAttributes<HTMLSpanElement> {
    // success, warning, danger and info are colored; neutral (the default) is the plain badge.
    tone?: "neutral" | "success" | "warning" | "danger" | "info";
    // Shows a small leading dot in the tone's color.
    dot?: boolean;
    children?: Snippet;
  }

  let { tone = "neutral", dot = true, class: className, children, ...rest }: Props = $props();
</script>

<span
  data-slot="badge"
  class={["status-badge", className]}
  data-tone={tone}
  data-dot={dot ? "" : undefined}
  {...rest}
>
  {@render children?.()}
</span>
