<script lang="ts">
 import { onMount } from 'svelte';
 import { Button, EmptyState, SearchInput, SelectDropdown, ThemeToggle } from '@kenn-io/kit-ui';
 import WorkflowRow from '$lib/components/ci/WorkflowRow.svelte';
 import SourceNotices from '$lib/components/ci/SourceNotices.svelte';
 import { parseOverview, filterWorkflows, groupWorkflows, readFilters, writeFilters, states, statusLabel, type RunState, type Filters, type Overview } from '$lib/ci';
 let overview = $state<Overview | null>(null);
 let filters = $state<Filters>(readFilters(new URLSearchParams(location.search)));
 let loading = $state(true);
 let error = $state('');
 let now = $state(Date.now());

 let request: AbortController | null = null;
 let timer: ReturnType<typeof setTimeout> | null = null;
 let disposed = false;
 let failures = 0;
 let visible = $derived(filterWorkflows(overview?.workflows ?? [], filters));
 let groups = $derived(groupWorkflows(visible));
 let repositories = $derived([...new Map((overview?.workflows ?? []).map(row => [row.repository.key, row.repository])).values()].sort((a,b) => `${a.source}/${a.full_name}`.localeCompare(`${b.source}/${b.full_name}`)));
 let providers = $derived([...new Set((overview?.sources ?? []).map(source => source.provider))].sort());
 let activeFilters = $derived(Object.values(filters).some(Boolean));
 let unavailableSources = $derived(overview?.sources.some(source => source.state !== 'healthy' && source.state !== 'ok' && source.state !== 'loading') ?? false);
 let pendingSources = $derived(overview?.sources.some(source => source.state === 'loading') ?? false);
 let stale = $derived(!!overview && overview.workflows.some(row => row.health.state !== 'healthy' && row.health.state !== 'ok' && row.health.state !== 'loading'));
 function updateFilter(key: keyof Filters, value: string, replace = false) {
  filters = { ...filters, [key]: value }; const url = new URL(location.href); url.search = writeFilters(filters, url.searchParams).toString();
  if (url.href !== location.href) history[replace ? 'replaceState' : 'pushState']({}, '', url);
 }
 function clearFilters() { filters = readFilters(new URLSearchParams()); const url = new URL(location.href); url.search = writeFilters(filters, url.searchParams).toString(); history.pushState({}, '', url); }
 async function load() {
  if (disposed) return; if (timer) clearTimeout(timer); request?.abort();
  const controller = new AbortController(); request = controller; loading = true;
  const timeout = setTimeout(() => controller.abort(), 15000);
  try {
   const response = await fetch('/api/overview', { signal: controller.signal, cache: 'no-store', headers: { Accept: 'application/json' } });
   if (!response.ok) throw new Error(`CI overview could not be loaded (HTTP ${response.status}).`);
   const next = parseOverview(await response.json()); if (request !== controller || disposed) return;
   overview = next; now = Date.now(); error = ''; failures = 0;
  } catch (cause) {
   if (request !== controller || disposed) return;
   error = controller.signal.aborted ? 'The overview request timed out. Try again.' : cause instanceof SyntaxError ? 'The server returned unreadable data. Try again.' : cause instanceof Error ? cause.message : 'CI overview could not be loaded. Try again.';
   failures++;
  } finally {
   clearTimeout(timeout);
   if (request === controller && !disposed) { loading = false; const interval = (overview?.poll_interval_seconds ?? 60) * 1000; timer = setTimeout(load, Math.min(300000, interval * 2 ** Math.min(failures, 3))); }
  }
 }
 onMount(() => {
  void load(); const tick = setInterval(() => now = Date.now(), 30000);
  const navigate = () => filters = readFilters(new URLSearchParams(location.search)); window.addEventListener('popstate', navigate);
  return () => { disposed = true; request?.abort(); if (timer) clearTimeout(timer); clearInterval(tick); window.removeEventListener('popstate', navigate); };
 });
</script>
<svelte:head><meta name="description" content="A read-only overview of GitHub and Forgejo CI."/></svelte:head>
<main>
 <header class="page-header">
  <h1>CI View</h1>
  <div class="refresh-controls"><span class="refresh-status" role="status">{#if loading}{overview ? 'Refreshing…' : 'Loading…'}{:else if error}Overview unavailable{:else if stale}Some data is stale{/if}</span><ThemeToggle/><Button surface="outline" class="refresh-button" disabled={loading} onclick={() => void load()} title="Reload the cached overview">Refresh</Button></div>
 </header>
 <section class="filters" aria-label="Workflow filters">
  <div class="search-label"><SearchInput block ariaLabel="Search workflows and repositories" placeholder="Search workflows or repositories…" value={filters.search} oninput={(value) => updateFilter('search', value, true)} /></div>
  <SelectDropdown title="Provider" value={filters.provider} onchange={(value) => updateFilter('provider', value)} options={[
   { value: '', label: 'All providers' },
   ...providers.map(provider => ({ value: provider, label: provider === 'github' ? 'GitHub' : provider === 'forgejo' ? 'Forgejo' : provider })),
   ...(filters.provider && !providers.includes(filters.provider) ? [{ value: filters.provider, label: `${filters.provider} (unavailable)` }] : []),
  ]}/>
  <SelectDropdown title="Source" value={filters.source} onchange={(value) => updateFilter('source', value)} options={[
   { value: '', label: 'All sources' },
   ...(overview?.sources ?? []).map(source => ({ value: source.name, label: source.name })),
   ...(filters.source && !overview?.sources.some(source => source.name === filters.source) ? [{ value: filters.source, label: `${filters.source} (unavailable)` }] : []),
  ]}/>
  <SelectDropdown class="repository-filter" title="Repository" value={filters.repository} onchange={(value) => updateFilter('repository', value)} options={[
   { value: '', label: 'All repositories' },
   ...repositories.map(repo => ({ value: repo.key, label: `${repo.source} / ${repo.full_name}` })),
   ...(filters.repository && !repositories.some(repo => repo.key === filters.repository) ? [{ value: filters.repository, label: 'Selected repository (unavailable)' }] : []),
  ]}/>
  <SelectDropdown title="Latest status" value={filters.status} onchange={(value) => updateFilter('status', value)} options={[
   { value: '', label: 'All statuses' },
   ...states.map(state => ({ value: state, label: statusLabel(state as RunState) })),
  ]}/>
  {#if activeFilters}<div class="filter-actions"><Button surface="outline" class="clear-filters" onclick={clearFilters}>Clear filters</Button></div>{/if}
 </section>
 {#if error}<div class="request-notice" role="alert"><div><strong>{overview ? 'Showing the last overview' : 'Unable to load workflows'}</strong><p>{error}</p></div><Button onclick={() => void load()} disabled={loading}>{loading ? 'Retrying…' : 'Retry'}</Button></div>{/if}
 <section class="workflow-overview" aria-label="Workflows" aria-busy={loading && !overview}>
  {#if !overview && loading}<div class="workflow-state loading-state" role="status"><span class="sr-only">Loading workflow overview</span>{#each [1,2,3,4] as row (row)}<div class="skeleton-row" aria-hidden="true"><div></div><div></div><div></div></div>{/each}</div>
  {:else if overview && !overview.workflows.length}<div class="workflow-state empty-state"><EmptyState title={pendingSources ? 'Loading repositories' : unavailableSources ? 'Workflow data unavailable' : 'No observed workflows'} description={pendingSources ? 'The server is collecting the first run sample.' : unavailableSources ? 'Check the source notices below for unavailable selections or connection errors.' : 'No workflow runs were found in the configured repositories and branch scope.'}/></div>
  {:else if overview && !visible.length}<div class="workflow-state empty-state"><EmptyState title="No matching workflows" description="Try another search or clear your filters."><Button surface="outline" onclick={clearFilters}>Clear filters</Button></EmptyState></div>
  {:else if overview}
   <div class="source-groups">
    {#each groups as source (source.name)}
     <section class="source-group" aria-label={source.name}>
      <h2 class="source-heading">{source.name}</h2>
      <div class="repository-groups">
       {#each source.repositories as group (group.repository.key)}
        <section class="repository-group" aria-label={group.repository.full_name}>
         <div class="repository-panel">
          {#each group.workflows as workflow (workflow.key)}<WorkflowRow {workflow} {now}/>{/each}
         </div>
        </section>
       {/each}
      </div>
     </section>
    {/each}
   </div>
  {:else}<div class="workflow-state empty-state"><EmptyState title="Waiting for connection" description="Your workflow overview will appear when the connection recovers."/></div>{/if}
 </section>
 {#if overview}<SourceNotices sources={overview.sources}/>{/if}
</main>
