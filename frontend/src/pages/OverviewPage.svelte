<script lang="ts">
  import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
  import { EmptyState } from "@kenn-io/kit-ui";
  import ActivityPanel from "$lib/components/ci/ActivityPanel.svelte";
  import StatStrip, { type Stat } from "$lib/components/ci/StatStrip.svelte";
  import {
    overviewAlerts,
    relativeTime,
    runnerCounts,
    workflowCounts,
    type Activity,
    type Alert,
    type Overview,
    type RunnerScope,
  } from "$lib/ci";
  let {
    overview,
    runners,
    activity,
    now,
    loading,
    link,
    onnavigate,
  }: {
    overview: Overview | null;
    runners: RunnerScope[];
    activity: Activity;
    now: number;
    loading: boolean;
    link: (href: string) => string;
    onnavigate: (href: string) => void;
  } = $props();
  const alertLimit = 3;
  const plural = (n: number, one: string, many = `${one}s`) => `${n} ${n === 1 ? one : many}`;
  let workflows = $derived(overview?.workflows ?? []);
  let counts = $derived(runnerCounts(runners));
  let wf = $derived(workflowCounts(workflows));
  let repositories = $derived(new Set(workflows.map((row) => row.repository.key)).size);
  let alerts = $derived(
    overview ? overviewAlerts({ workflows, sources: overview.sources, runners, activity }) : [],
  );
  let runnerStats = $derived<Stat[]>([
    {
      label: "Working",
      value: counts.working,
      tone: "running",
      caption: `of ${counts.total} runners${counts.unknown ? ` · ${counts.unknown} unknown` : ""}`,
      bar: [
        { tone: "running", count: counts.working },
        { tone: "success", count: counts.idle },
        { tone: "failure", count: counts.offline },
        { tone: "neutral", count: counts.unknown },
      ],
    },
    { label: "Idle", value: counts.idle, tone: "success", caption: "online, ready for jobs" },
    { label: "Offline", value: counts.offline, tone: "failure", caption: "per GitHub" },
    {
      label: "Queued jobs",
      value: activity.queued.length,
      tone: "waiting",
      caption: "waiting to start",
    },
  ]);
  let workflowStats = $derived<Stat[]>([
    { label: "Failing", value: wf.failing, tone: "failure", caption: "latest run failed" },
    {
      label: "In progress",
      value: wf.running + wf.queued,
      tone: "running",
      caption: `${wf.running} running · ${wf.queued} queued`,
    },
    {
      label: "Passing",
      value: wf.passing,
      tone: "success",
      caption: `of ${wf.total} workflows`,
      bar: [
        { tone: "failure", count: wf.failing },
        { tone: "running", count: wf.running + wf.queued },
        { tone: "success", count: wf.passing },
        { tone: "neutral", count: wf.other },
      ],
    },
    { label: "Other", value: wf.other, tone: "neutral", caption: "canceled, skipped or no runs" },
  ]);
</script>

{#snippet alertRow(alert: Alert)}
  <div class="alert-row" data-tone={alert.tone}>
    <TriangleAlert size={16} strokeWidth={1.75} aria-hidden="true" />
    <div class="alert-body">
      <p class="alert-title">
        {#if alert.href}<a href={alert.href} target="_blank" rel="noopener noreferrer"
            >{alert.title}</a
          >{:else}{alert.title}{/if}
        <span class="alert-kind">{alert.kind}</span>
      </p>
      <p class="alert-detail">
        {alert.detail}{#if alert.since}{" · "}Since {relativeTime(alert.since, now)}{/if}
      </p>
    </div>
  </div>
{/snippet}

{#if !overview && !loading}
  <div class="workflow-state empty-state">
    <EmptyState
      title="Waiting for connection"
      description="Your overview will appear when the connection recovers."
    />
  </div>
{:else if !overview}
  <p class="overview-subtitle" role="status">Loading…</p>
{:else}
  <header class="overview-head">
    <h2 class="overview-title">Overview</h2>
    <p class="overview-subtitle">
      {plural(wf.total, "workflow")} · {plural(repositories, "repository", "repositories")}{counts.total ||
      runners.length
        ? ` · ${plural(counts.total, "runner")}`
        : ""} · Updated {relativeTime(overview.generated_at, now)}
    </p>
  </header>
  {#if runners.length}<StatStrip label="Runners" stats={runnerStats} />{/if}
  <StatStrip label="Workflows" stats={workflowStats} />
  {#if alerts.length}
    <section class="overview-section" aria-label="Active alerts">
      <h3 class="section-title">Active alerts <span class="count">{alerts.length}</span></h3>
      <div class="alert-list">
        {#each alerts.slice(0, alertLimit) as alert (alert.key)}{@render alertRow(alert)}{/each}
        {#if alerts.length > alertLimit}
          <details class="alert-more">
            <summary>Show {alerts.length - alertLimit} more</summary>
            {#each alerts.slice(alertLimit) as alert (alert.key)}{@render alertRow(alert)}{/each}
          </details>
        {/if}
      </div>
    </section>
  {/if}
  {#if activity.running.length || activity.queued.length}
    <section class="overview-section" aria-label="Activity">
      <ActivityPanel {activity} {now} limit={4} {link} {onnavigate} />
    </section>
  {/if}
{/if}
