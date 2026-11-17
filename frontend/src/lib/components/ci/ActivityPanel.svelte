<script lang="ts">
  import ChevronRight from "@lucide/svelte/icons/chevron-right";
  import { followLink } from "$lib/route";
  import { formatDuration, safeURL, type Activity, type ActivityItem } from "$lib/ci";
  let {
    activity,
    now,
    limit = 5,
    link,
    onnavigate,
  }: {
    activity: Activity;
    now: number;
    limit?: number;
    link: (href: string) => string;
    onnavigate: (href: string) => void;
  } = $props();
  const elapsed = (item: ActivityItem) => formatDuration(Math.max(0, now - Date.parse(item.since)));
  const lists = $derived([
    { id: "running", title: "Working now", items: activity.running, status: "running" },
    { id: "queued", title: "Queued", items: activity.queued, status: "queued" },
  ]);
  const place = (item: ActivityItem) =>
    item.runner ??
    (item.pool === "github-hosted" ? "GitHub-hosted" : item.pool ? "Self-hosted" : "");
</script>

<div class="activity-grid">
  {#each lists as list (list.id)}
    {@const shown = list.items.slice(0, limit)}
    {@const more = list.items.length - shown.length}
    <section aria-label={list.title}>
      <h3 class="section-title">{list.title} <span class="count">{list.items.length}</span></h3>
      <div class="repository-panel activity-list">
        {#each shown as item (item.key)}
          {@const url = safeURL(item.url)}
          <article class="activity-row" aria-label={`${item.repository}, ${item.workflow}`}>
            <i
              class="activity-dot"
              data-tone={list.id === "running" ? "running" : "waiting"}
              aria-hidden="true"
            ></i>
            <div class="activity-main">
              <span class="activity-title"
                >{#if url}<a href={url} target="_blank" rel="noopener noreferrer">{item.workflow}</a
                  >{:else}{item.workflow}{/if}</span
              >
              <span class="activity-meta"
                >{item.repository} · {item.branch}{place(item) ? ` · ${place(item)}` : ""}</span
              >
              {#if item.hint}<span class="activity-hint" data-tone={item.hint.tone}
                  >{item.hint.text}</span
                >{/if}
            </div>
            <span class="activity-time"
              >{list.id === "queued" ? "waiting " : ""}{elapsed(item)}</span
            >
          </article>
        {:else}
          <p class="runner-empty">Nothing {list.id === "running" ? "running" : "queued"}</p>
        {/each}
        {#if more > 0}
          {@const href = link(`/workflows?status=${list.status}`)}
          <a class="activity-more" {href} onclick={(event) => followLink(event, onnavigate)}
            >{more} more<ChevronRight size={14} aria-hidden="true" /></a
          >
        {/if}
      </div>
    </section>
  {/each}
</div>
