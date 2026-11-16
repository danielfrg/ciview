<script lang="ts">
  import { Chip } from "@kenn-io/kit-ui";
  import {
    absoluteTime,
    relativeTime,
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
            </div>
            <div class="runner-state">
              <Chip tone={runnerTone(runner, scope.state)} dot uppercase={false}
                >{runnerLabel(runner)}</Chip
              >
            </div>
            <div class="runner-labels">{runner.labels.join(" · ")}</div>
            <div class="runner-version">{runner.version ? `v${runner.version}` : "—"}</div>
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
