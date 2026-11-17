<script lang="ts">
  import { app } from "../lib/state.svelte";
  import Icon from "./Icon.svelte";
  import ServiceIcon from "./ServiceIcon.svelte";
  import type { Container } from "../lib/api";

  const services = $derived(app.metrics?.services ?? []);
  const ports = $derived(app.metrics?.ports ?? []);
  const dockerUp = $derived(services.some((s) => s.id === "docker" && s.running));
  /** null until the first listing arrives (or while Docker is down). */
  const containers = $derived(app.metrics?.containers ?? null);
  const up = (c: Container) => c.state === "running" || c.state === "paused" || c.state === "restarting";
  const runningCount = $derived(containers?.filter(up).length ?? 0);
  /** Compose projects in listing order, then standalone containers. */
  const groups = $derived.by(() => {
    const projects = new Map<string, Container[]>();
    const solo: Container[] = [];
    for (const c of containers ?? []) {
      if (!c.project) solo.push(c);
      else projects.set(c.project, [...(projects.get(c.project) ?? []), c]);
    }
    return { projects: [...projects], solo };
  });

  // Names for listeners whose owner ss can't show (root daemons).
  const WELL_KNOWN: Record<number, string> = {
    80: "HTTP", 443: "HTTPS", 2375: "Docker API", 3306: "MySQL", 5432: "PostgreSQL",
    6379: "Redis", 8080: "HTTP", 9200: "Elasticsearch", 11211: "Memcached", 27017: "MongoDB",
  };
  // Databases speak their own protocol; offer copy instead of a browser.
  const NOT_HTTP = new Set([3306, 5432, 6379, 9200, 11211, 27017, 2375]);
</script>

<section>
  <div class="tabs" role="tablist" aria-label="Development">
    <button role="tab" aria-selected={app.tab === "services"} onclick={() => app.setTab("services")}>
      Services
    </button>
    <button role="tab" aria-selected={app.tab === "containers"} onclick={() => app.setTab("containers")}>
      Containers{#if runningCount}<span class="count">{runningCount}</span>{/if}
    </button>
    <button role="tab" aria-selected={app.tab === "ports"} onclick={() => app.setTab("ports")}>
      Ports{#if ports.length}<span class="count">{ports.length}</span>{/if}
    </button>
  </div>

  {#if app.tab === "services"}
    {#if services.length === 0}
      <p class="empty">No watched services. Pick some in Settings.</p>
    {:else}
      <ul>
        {#each services as s (s.id)}
          <li>
            <ServiceIcon id={s.id} dim={!s.running} />
            <span class="name">{s.name}</span>
            {#if s.inContainer}
              <span class="detail" title="This {s.name} is part of a Docker container, not a service installed in Linux">
                in {s.container || "a container"}
              </span>
            {:else if s.detail}<span class="detail">{s.detail}</span>{/if}
            <span class="state" class:on={s.running}>{s.running ? "Running" : "Stopped"}</span>
            {#if app.serviceBusy[s.id]}
              <span class="row-btn"><span class="spin"></span></span>
            {:else if s.inContainer}
              <button
                class="row-btn"
                title="Runs inside {s.container || 'a container'}. Manage it in Containers"
                aria-label="Show {s.container || 'container'} in Containers"
                onclick={() => app.setTab("containers")}
              >
                <Icon name="box" size={13} />
              </button>
            {:else if s.controllable}
              <button
                class="row-btn"
                title={s.running ? `Stop ${s.name}` : `Start ${s.name}`}
                aria-label={s.running ? `Stop ${s.name}` : `Start ${s.name}`}
                onclick={() => app.serviceControl(s.id, s.running ? "stop" : "start")}
              >
                <Icon name={s.running ? "stop" : "play"} size={12} />
              </button>
            {:else}
              <span class="row-btn" title={s.id === "tailscale-ssh"
                ? "Turn on with: sudo tailscale set --ssh"
                : "Managed by Docker Desktop"}></span>
            {/if}
          </li>
        {/each}
      </ul>
    {/if}
  {:else if app.tab === "containers"}
    {#if !dockerUp}
      <p class="empty">Docker is not running. Start it in Services.</p>
    {:else if app.metrics?.containersError}
      <p class="empty">{app.metrics.containersError}</p>
    {:else if containers === null}
      <p class="empty loading"><span class="spin"></span> Listing containers…</p>
    {:else if containers.length === 0}
      <p class="empty">No containers yet.</p>
    {:else}
      <ul class="scroll">
        {#each groups.projects as [project, list] (project)}
          {@const n = list.filter(up).length}
          {@const target = `project:${project}`}
          <li class="group">
            <Icon name="folder" size={13} />
            <span class="name">{project}</span>
            <span class="state" class:on={n > 0}>{n}/{list.length} running</span>
            {#if app.containerBusy[target]}
              <span class="row-btn"><span class="spin"></span></span>
              <span class="row-btn"></span>
            {:else}
              <button
                class="row-btn"
                title="Restart {project}"
                aria-label="Restart {project}"
                onclick={() => app.containerAction(target, "restart")}
              >
                <Icon name="restart" size={12} />
              </button>
              <button
                class="row-btn"
                title={n > 0 ? `Stop ${project}` : `Start ${project}`}
                aria-label={n > 0 ? `Stop ${project}` : `Start ${project}`}
                onclick={() => app.containerAction(target, n > 0 ? "stop" : "start")}
              >
                <Icon name={n > 0 ? "stop" : "play"} size={12} />
              </button>
            {/if}
          </li>
          {#each list as c (c.id)}
            {@render row(c, true)}
          {/each}
        {/each}
        {#each groups.solo as c (c.id)}
          {@render row(c, false)}
        {/each}
      </ul>
    {/if}
  {:else if ports.length === 0}
    <p class="empty">Nothing is listening inside Linux right now.</p>
  {:else}
    <ul>
      {#each ports as p (p.port)}
        {@const label = p.process || WELL_KNOWN[p.port] || ""}
        <li>
          <span class="port">:{p.port}</span>
          <span class="name">{label}</span>
          {#if p.loopback}<span class="detail" title="Bound to 127.0.0.1 inside Linux">local</span>{/if}
          {#if NOT_HTTP.has(p.port)}
            <button class="row-btn wide" onclick={() => app.copyAddress(p.port)} title="Copy localhost:{p.port}">
              <Icon name="copy" size={13} />
            </button>
          {:else}
            <button class="row-btn wide" onclick={() => app.openPort(p.port)} title="Open http://localhost:{p.port}">
              <Icon name="external" size={13} />
            </button>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
</section>

{#snippet row(c: Container, nested: boolean)}
  {@const on = up(c)}
  <li class="ctr" class:nested title="{c.name} · {c.image} · {c.status}{c.ports ? ` · ${c.ports}` : ''}">
    <i class:on></i>
    <span class="name">{nested && c.service ? c.service : c.name}</span>
    <span class="detail">{c.image}</span>
    <span class="state" class:on>{on ? c.status || c.state : c.state === "exited" ? "Exited" : c.state}</span>
    {#if app.containerBusy[c.id] || app.containerBusy[`project:${c.project}`]}
      <span class="row-btn"><span class="spin"></span></span>
      <span class="row-btn"></span>
    {:else}
      <button
        class="row-btn"
        title="Restart {c.name}"
        aria-label="Restart {c.name}"
        disabled={!on}
        onclick={() => app.containerAction(c.id, "restart")}
      >
        <Icon name="restart" size={12} />
      </button>
      <button
        class="row-btn"
        title={on ? `Stop ${c.name}` : `Start ${c.name}`}
        aria-label={on ? `Stop ${c.name}` : `Start ${c.name}`}
        onclick={() => app.containerAction(c.id, on ? "stop" : "start")}
      >
        <Icon name={on ? "stop" : "play"} size={12} />
      </button>
    {/if}
  </li>
{/snippet}

<style>
  .tabs {
    display: flex;
    gap: 2px;
    margin: 0 0 6px;
  }
  .tabs button {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    height: 26px;
    padding: 0 8px;
    border: 0;
    border-radius: 6px;
    background: transparent;
    color: var(--text-2);
    font-weight: 600;
    font-size: 12px;
    position: relative;
  }
  .tabs button:hover {
    background: var(--card-hover);
    color: var(--text);
  }
  .tabs button[aria-selected="true"] {
    color: var(--text);
  }
  .tabs button[aria-selected="true"]::after {
    content: "";
    position: absolute;
    left: 8px;
    right: 8px;
    bottom: 1px;
    height: 2px;
    border-radius: 2px;
    background: var(--accent);
  }
  .count {
    min-width: 16px;
    padding: 0 4px;
    border-radius: 8px;
    background: var(--accent-soft);
    color: var(--accent);
    font-size: 10.5px;
    line-height: 16px;
    text-align: center;
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    background: var(--card);
    border: 1px solid var(--stroke);
    border-radius: var(--radius);
    box-shadow: var(--shadow);
  }
  li {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 36px;
    padding: 4px 6px 4px 12px;
  }
  li + li {
    border-top: 1px solid var(--stroke);
  }
  i {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--idle-dot);
    flex: none;
  }
  i.on {
    background: var(--ok-dot);
  }
  .name {
    font-weight: 500;
    white-space: nowrap;
  }
  .port {
    font: 600 12.5px ui-monospace, "Cascadia Mono", Consolas, monospace;
    min-width: 46px;
  }
  .detail {
    color: var(--text-3);
    font-size: 11px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .state {
    margin-left: auto;
    color: var(--text-3);
    font-size: 12px;
  }
  .state.on {
    color: var(--ok);
  }
  .row-btn {
    display: grid;
    place-items: center;
    width: 26px;
    height: 26px;
    flex: none;
    border: 0;
    border-radius: 5px;
    background: transparent;
    color: var(--text-2);
  }
  .row-btn.wide {
    margin-left: auto;
  }
  button.row-btn:hover {
    background: var(--card-hover);
    color: var(--text);
  }
  .spin {
    width: 12px;
    height: 12px;
    border-radius: 50%;
    border: 2px solid var(--track);
    border-top-color: var(--accent);
    animation: spin 0.8s linear infinite;
  }
  @keyframes spin {
    to {
      rotate: 360deg;
    }
  }
  /* About eight rows, then the list scrolls instead of growing the flyout. */
  ul.scroll {
    max-height: 292px;
    overflow-y: auto;
  }
  li.group {
    color: var(--text-2);
    background: var(--card-hover);
  }
  li.group .name {
    font-weight: 600;
    color: var(--text);
  }
  /* The name keeps its width; the image only gets what is left over. */
  li.ctr .name {
    flex: 0 1 auto;
    min-width: 40px;
  }
  li.ctr .detail {
    flex: 1 1 0;
    min-width: 0;
  }
  li.nested {
    padding-left: 24px;
  }
  .state {
    white-space: nowrap;
    flex: none;
  }
  .name {
    overflow: hidden;
    text-overflow: ellipsis;
  }
  button.row-btn:disabled {
    opacity: 0.35;
  }
  button.row-btn:disabled:hover {
    background: transparent;
    color: var(--text-2);
  }
  .empty.loading {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .empty {
    margin: 0;
    padding: 10px 12px;
    color: var(--text-3);
    background: var(--card);
    border: 1px dashed var(--stroke-strong);
    border-radius: var(--radius);
  }
</style>
