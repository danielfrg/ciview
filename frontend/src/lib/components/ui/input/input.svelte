<script lang="ts">
 import type { HTMLInputAttributes } from 'svelte/elements';
 import { cn } from '$lib/utils';
 let { class: className, value = $bindable(''), ...props }: HTMLInputAttributes = $props();
</script>
<input data-slot="input" class={cn('h-11 min-w-0 rounded-md border border-input bg-card px-3 text-sm text-foreground placeholder:text-muted-foreground focus-visible:border-ring focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:opacity-50', className)} bind:value {...props} />
