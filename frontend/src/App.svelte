<script lang="ts">
  import { onMount } from "svelte";
  import Activity from "@lucide/svelte/icons/activity";
  import { Button } from "@ui-sdk/svelte/button";
  import { Preferences } from "@ui-sdk/svelte/preferences";
  import { ThemeToggle } from "./design-system/custom/theme-toggle";
  import { colorScheme } from "$lib/color-scheme";
  import OverviewPage from "./pages/OverviewPage.svelte";
  import RunnersPage from "./pages/RunnersPage.svelte";
  import WorkflowsPage from "./pages/WorkflowsPage.svelte";
  import { activityFromWorkflows, parseOverview, type Overview } from "$lib/ci";
  import { followLink, pages, routeFor, withMock } from "$lib/route";
  let overview = $state<Overview | null>(null);
  let loading = $state(true);
  let error = $state("");
  let now = $state(Date.now());
  let path = $state(location.pathname);
  let route = $derived(routeFor(path));

  let request: AbortController | null = null;
  let timer: ReturnType<typeof setTimeout> | null = null;
  let disposed = false;
  let failures = 0;
  // Dev-only preview of runner and activity UI with fictional data: open the dev server with ?mock.
  let mock = $state<typeof import("$lib/mock") | null>(null);
  let runnerScopes = $derived(mock ? mock.mockRunners(now) : (overview?.runners ?? []));
  let activity = $derived(
    mock ? mock.mockActivity(now) : activityFromWorkflows(overview?.workflows ?? []),
  );
  let stale = $derived(
    !!overview &&
      overview.workflows.some(
        (row) =>
          row.health.state !== "healthy" &&
          row.health.state !== "ok" &&
          row.health.state !== "loading",
      ),
  );
  const link = (href: string) => withMock(href, !!mock);
  function go(href: string) {
    history.pushState({}, "", href);
    path = location.pathname;
    window.scrollTo(0, 0);
  }
  async function load() {
    if (disposed) return;
    if (timer) clearTimeout(timer);
    request?.abort();
    const controller = new AbortController();
    request = controller;
    loading = true;
    const timeout = setTimeout(() => controller.abort(), 15000);
    try {
      const response = await fetch("/api/overview", {
        signal: controller.signal,
        cache: "no-store",
        headers: { Accept: "application/json" },
      });
      if (!response.ok)
        throw new Error(`CI overview could not be loaded (HTTP ${response.status}).`);
      const next = parseOverview(await response.json());
      if (request !== controller || disposed) return;
      overview = next;
      now = Date.now();
      error = "";
      failures = 0;
    } catch (cause) {
      if (request !== controller || disposed) return;
      error = controller.signal.aborted
        ? "The overview request timed out. Try again."
        : cause instanceof SyntaxError
          ? "The server returned unreadable data. Try again."
          : cause instanceof Error
            ? cause.message
            : "CI overview could not be loaded. Try again.";
      failures++;
    } finally {
      clearTimeout(timeout);
      if (request === controller && !disposed) {
        loading = false;
        const interval = (overview?.poll_interval_seconds ?? 60) * 1000;
        timer = setTimeout(load, Math.min(300000, interval * 2 ** Math.min(failures, 3)));
      }
    }
  }
  onMount(() => {
    if (import.meta.env.DEV && new URLSearchParams(location.search).has("mock"))
      void import("$lib/mock").then((module) => (mock = module));
    void load();
    const tick = setInterval(() => (now = Date.now()), 30000);
    const navigate = () => (path = location.pathname);
    window.addEventListener("popstate", navigate);
    return () => {
      disposed = true;
      request?.abort();
      if (timer) clearTimeout(timer);
      clearInterval(tick);
      window.removeEventListener("popstate", navigate);
    };
  });
</script>

<svelte:head
  ><meta name="description" content="A read-only overview of GitHub and Forgejo CI." /></svelte:head
>
<Preferences preferences={[colorScheme]}>
  <main>
    <header class="page-header">
      <div class="page-brand">
        <h1><Activity size={20} strokeWidth={1.5} aria-hidden="true" />CI View</h1>
        <nav class="page-nav" aria-label="Pages">
          {#each pages as page (page.route)}
            <a
              href={link(page.path)}
              aria-current={route === page.route ? "page" : undefined}
              onclick={(event) => followLink(event, go)}>{page.label}</a
            >
          {/each}
        </nav>
      </div>
      <div class="refresh-controls">
        <span class="refresh-status" role="status"
          >{#if loading}{overview ? "Refreshing…" : "Loading…"}{:else if error}Overview unavailable{:else if stale}Some
            data is stale{/if}</span
        ><ThemeToggle {colorScheme} /><Button
          data-variant="outline"
          class="refresh-button"
          disabled={loading}
          onclick={() => void load()}
          title="Reload the cached overview">Refresh</Button
        >
      </div>
    </header>
    {#if mock}<p class="mock-banner" role="status">
        Mock data: runners and activity are fictional (dev preview only)
      </p>{/if}
    {#if error}<div class="request-notice" role="alert">
        <div>
          <strong>{overview ? "Showing the last overview" : "Unable to load workflows"}</strong>
          <p>{error}</p>
        </div>
        <Button onclick={() => void load()} disabled={loading}
          >{loading ? "Retrying…" : "Retry"}</Button
        >
      </div>{/if}
    {#if route === "workflows"}
      <WorkflowsPage {overview} {loading} {now} />
    {:else if route === "runners"}
      <RunnersPage scopes={runnerScopes} {now} {loading} />
    {:else}
      <OverviewPage
        {overview}
        runners={runnerScopes}
        {activity}
        {now}
        {loading}
        {link}
        onnavigate={go}
      />
    {/if}
  </main>
</Preferences>
