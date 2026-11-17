<script lang="ts">
  import StatusBadge from "$lib/components/StatusBadge.svelte";
  import {
    absoluteTime,
    formatDuration,
    relativeTime,
    safeURL,
    runnerLabel,
    runnerScopeNotice,
    runnerTone,
    type RunnerScope,
  } from "$lib/ci";
  let { scopes, now }: { scopes: RunnerScope[]; now: number } = $props();
</script>

<section class="source-group runner-group" aria-label="Runners">
  <h2 class="source-heading">Runners</h2>
  <div class="repository-groups">
    {#each scopes as scope (`${scope.source}/${scope.scope}`)}
      {@const notice = runnerScopeNotice(scope)}
      <section class="repository-panel runner-scope" aria-label={`${scope.scope} runners`}>
        <header class="runner-scope-head">
          <h3 class="runner-scope-name">{scope.scope}</h3>
          {#if scope.as_of}<small title={absoluteTime(scope.as_of)}
              >Updated {relativeTime(scope.as_of, now)}</small
            >{/if}
        </header>
        {#if notice}<p class="row-notice runner-notice">{notice}</p>{/if}
        {#each scope.runners as runner (runner.id)}
          <article class="runner-row" aria-label={`${scope.scope}, ${runner.name}`}>
            <div class="runner-identity">
              <span class="runner-name">{runner.name}</span>
              {#if runner.os}<span class="runner-os">{runner.os}</span>{/if}
              {#if runner.job}{@const jobURL = safeURL(runner.job.url)}<span class="runner-job"
                  >Working on {#if jobURL}<a href={jobURL} target="_blank" rel="noopener noreferrer"
                      >{runner.job.title}</a
                    >{:else}{runner.job.title}{/if} · {formatDuration(
                    Math.max(0, now - Date.parse(runner.job.started_at)),
                  )}</span
                >{/if}
            </div>
            <div class="runner-state">
              <StatusBadge tone={runnerTone(runner, scope.state)} dot
                >{runnerLabel(runner)}</StatusBadge
              >
              {#if runner.update}<StatusBadge
                  tone={runner.update.overdue ? "danger" : "warning"}
                  title={`v${runner.update.latest} is available. Missing a release since ${absoluteTime(runner.update.missing_since)}.${runner.update.overdue ? " GitHub may stop sending jobs to a runner more than 30 days behind." : ""}`}
                  >{runner.update.overdue ? "Update overdue" : "Update available"}</StatusBadge
                >{/if}
            </div>
            <div class="runner-labels">{runner.labels.join(" · ")}</div>
            <div class="runner-version">
              {runner.version ? `v${runner.version}` : "—"}{#if runner.update}<small
                  >→ v{runner.update.latest}</small
                >{/if}
            </div>
          </article>
        {:else}
          {#if scope.state === "healthy" || scope.state === "ok"}<p class="runner-empty">
              No runners registered
            </p>{/if}
        {/each}
      </section>
    {/each}
  </div>
</section>
