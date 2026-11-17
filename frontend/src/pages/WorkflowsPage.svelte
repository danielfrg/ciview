<script lang="ts">
  import { onMount } from "svelte";
  import XIcon from "@lucide/svelte/icons/x";
  import SearchIcon from "@lucide/svelte/icons/search";
  import ChevronDownIcon from "@lucide/svelte/icons/chevron-down";
  import CheckIcon from "@lucide/svelte/icons/check";
  import { Button } from "@ui-sdk/svelte/button";
  import { Input, InputClear, InputGroup } from "@ui-sdk/svelte/input";
  import {
    Select,
    SelectContent,
    SelectItem,
    SelectItemIndicator,
    SelectLabel,
    SelectTrigger,
    SelectValue,
  } from "@ui-sdk/svelte/select";
  import EmptyState from "$lib/components/EmptyState.svelte";
  import WorkflowRow from "$lib/components/ci/WorkflowRow.svelte";
  import SourceNotices from "$lib/components/ci/SourceNotices.svelte";
  import {
    filterWorkflows,
    groupWorkflows,
    readFilters,
    writeFilters,
    states,
    statusLabel,
    type RunState,
    type Filters,
    type Overview,
  } from "$lib/ci";
  let { overview, loading, now }: { overview: Overview | null; loading: boolean; now: number } =
    $props();
  let filters = $state<Filters>(readFilters(new URLSearchParams(location.search)));
  let visible = $derived(filterWorkflows(overview?.workflows ?? [], filters));
  let groups = $derived(groupWorkflows(visible, filters.sort));
  let repositories = $derived(
    [
      ...new Map(
        (overview?.workflows ?? []).map((row) => [row.repository.key, row.repository]),
      ).values(),
    ].sort((a, b) => `${a.source}/${a.full_name}`.localeCompare(`${b.source}/${b.full_name}`)),
  );
  let providers = $derived(
    [...new Set((overview?.sources ?? []).map((source) => source.provider))].sort(),
  );
  let multipleSources = $derived((overview?.sources.length ?? 0) > 1);
  let activeFilters = $derived(Object.values(filters).some(Boolean));
  let unavailableSources = $derived(
    overview?.sources.some(
      (source) => source.state !== "healthy" && source.state !== "ok" && source.state !== "loading",
    ) ?? false,
  );
  let pendingSources = $derived(
    overview?.sources.some((source) => source.state === "loading") ?? false,
  );
  type Option = { value: string; label: string };
  let selects = $derived<
    { key: keyof Filters; title: string; class?: string; options: Option[] }[]
  >([
    {
      key: "provider",
      title: "Provider",
      options: [
        { value: "", label: "All providers" },
        ...providers.map((provider) => ({
          value: provider,
          label: provider === "github" ? "GitHub" : provider === "forgejo" ? "Forgejo" : provider,
        })),
        ...(filters.provider && !providers.includes(filters.provider)
          ? [{ value: filters.provider, label: `${filters.provider} (unavailable)` }]
          : []),
      ],
    },
    {
      key: "source",
      title: "Source",
      options: [
        { value: "", label: "All sources" },
        ...(overview?.sources ?? []).map((source) => ({ value: source.name, label: source.name })),
        ...(filters.source && !overview?.sources.some((source) => source.name === filters.source)
          ? [{ value: filters.source, label: `${filters.source} (unavailable)` }]
          : []),
      ],
    },
    {
      key: "repository",
      title: "Repository",
      class: "repository-filter",
      options: [
        { value: "", label: "All repositories" },
        ...repositories.map((repo) => ({
          value: repo.key,
          label: `${repo.source} / ${repo.full_name}`,
        })),
        ...(filters.repository && !repositories.some((repo) => repo.key === filters.repository)
          ? [{ value: filters.repository, label: "Selected repository (unavailable)" }]
          : []),
      ],
    },
    {
      key: "status",
      title: "Latest status",
      options: [
        { value: "", label: "All statuses" },
        ...states.map((state) => ({ value: state, label: statusLabel(state as RunState) })),
      ],
    },
    {
      key: "sort",
      title: "Sort",
      options: [
        { value: "", label: "Newest run" },
        { value: "failing", label: "Failing first" },
        { value: "name", label: "By name" },
      ],
    },
  ]);
  function updateFilter(key: keyof Filters, value: string, replace = false) {
    filters = { ...filters, [key]: value };
    const url = new URL(location.href);
    url.search = writeFilters(filters, url.searchParams).toString();
    if (url.href !== location.href) history[replace ? "replaceState" : "pushState"]({}, "", url);
  }
  function clearFilters() {
    filters = readFilters(new URLSearchParams());
    const url = new URL(location.href);
    url.search = writeFilters(filters, url.searchParams).toString();
    history.pushState({}, "", url);
  }
  onMount(() => {
    const navigate = () => (filters = readFilters(new URLSearchParams(location.search)));
    window.addEventListener("popstate", navigate);
    return () => window.removeEventListener("popstate", navigate);
  });
</script>

<section class="filters" aria-label="Workflow filters">
  <InputGroup class="search-label">
    <SearchIcon aria-hidden="true" />
    <Input
      type="search"
      aria-label="Search workflows and repositories"
      placeholder="Search workflows…"
      autocomplete="off"
      bind:value={() => filters.search, (value) => updateFilter("search", value, true)}
    />
    <InputClear aria-label="Clear search"><XIcon aria-hidden="true" /></InputClear>
  </InputGroup>
  {#each selects as select (select.key)}
    <Select
      items={select.options}
      bind:value={() => filters[select.key], (value) => updateFilter(select.key, value ?? "")}
    >
      <SelectLabel class="sr-only">{select.title}</SelectLabel>
      <SelectTrigger class={select.class}>
        <SelectValue />
        <ChevronDownIcon aria-hidden="true" />
      </SelectTrigger>
      <SelectContent>
        {#each select.options as option (option.value)}
          <SelectItem value={option.value}>
            {option.label}
            <SelectItemIndicator><CheckIcon aria-hidden="true" /></SelectItemIndicator>
          </SelectItem>
        {/each}
      </SelectContent>
    </Select>
  {/each}
  <div class="filter-actions">
    <Button
      data-variant="outline"
      class="clear-filters"
      disabled={!activeFilters}
      onclick={clearFilters}>Clear filters</Button
    >
  </div>
</section>
<section class="workflow-overview" aria-label="Workflows" aria-busy={loading && !overview}>
  {#if !overview && loading}<div class="workflow-state loading-state" role="status">
      <span class="sr-only">Loading workflow overview</span>{#each [1, 2, 3, 4] as row (row)}<div
          class="skeleton-row"
          aria-hidden="true"
        >
          <div></div>
          <div></div>
          <div></div>
        </div>{/each}
    </div>
  {:else if overview && !overview.workflows.length}<div class="workflow-state empty-state">
      <EmptyState
        title={pendingSources
          ? "Loading repositories"
          : unavailableSources
            ? "Workflow data unavailable"
            : "No observed workflows"}
        description={pendingSources
          ? "The server is collecting the first run sample."
          : unavailableSources
            ? "Check the source notices below for unavailable selections or connection errors."
            : "No workflow runs were found in the configured repositories and branch scope."}
      />
    </div>
  {:else if overview && !visible.length}<div class="workflow-state empty-state">
      <EmptyState
        title="No matching workflows"
        description="Try another search or clear your filters."
        ><Button data-variant="outline" onclick={clearFilters}>Clear filters</Button></EmptyState
      >
    </div>
  {:else if overview}
    <div class="source-groups">
      {#each groups as source (source.name)}
        <section class="source-group" aria-label={source.name}>
          {#if multipleSources}<h2 class="source-heading">{source.name}</h2>{/if}
          <div class="repository-groups">
            {#each source.repositories as group (group.repository.key)}
              <section class="repository-group" aria-label={group.repository.full_name}>
                <div class="repository-panel">
                  {#each group.workflows as workflow (workflow.key)}<WorkflowRow
                      {workflow}
                      {now}
                    />{/each}
                </div>
              </section>
            {/each}
          </div>
        </section>
      {/each}
    </div>
  {:else}<div class="workflow-state empty-state">
      <EmptyState
        title="Waiting for connection"
        description="Your workflow overview will appear when the connection recovers."
      />
    </div>{/if}
</section>
{#if overview}<SourceNotices sources={overview.sources} />{/if}
