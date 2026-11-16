<script lang="ts" module>
  export type Tone = "running" | "success" | "failure" | "waiting" | "neutral";
  export type Stat = {
    label: string;
    value: number;
    tone: Tone;
    caption: string;
    // Segments of a proportional bar, as counts. Omit for no bar.
    bar?: { tone: Tone; count: number }[];
  };
</script>

<script lang="ts">
  let { label, stats }: { label: string; stats: Stat[] } = $props();
</script>

<section class="stat-strip" aria-label={label}>
  {#each stats as stat (stat.label)}
    {@const total = stat.bar?.reduce((sum, part) => sum + part.count, 0) ?? 0}
    <div class="stat">
      <span class="stat-label">{stat.label}</span>
      <span class="stat-value" data-tone={stat.value > 0 ? stat.tone : "neutral"}
        ><i class="stat-dot" aria-hidden="true"></i>{stat.value}</span
      >
      {#if stat.bar && total > 0}<span class="stat-bar" aria-hidden="true"
          >{#each stat.bar as part, i (i)}{#if part.count > 0}<i
                data-tone={part.tone}
                style:flex-grow={part.count}
              ></i>{/if}{/each}</span
        >{/if}
      <span class="stat-caption">{stat.caption}</span>
    </div>
  {/each}
</section>
