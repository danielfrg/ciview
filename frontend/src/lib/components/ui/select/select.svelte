<script lang="ts">
 import type { HTMLSelectAttributes } from 'svelte/elements';
 import { cn } from '$lib/utils';
 let { class: className, value = $bindable(''), children, ...props }: HTMLSelectAttributes = $props();
</script>
<select data-slot="select" class={cn('h-11 min-w-0 rounded-md border border-input bg-card px-3 text-sm text-foreground focus-visible:border-ring focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:opacity-50', className)} bind:value {...props}>{@render children?.()}</select>
