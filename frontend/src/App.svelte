<script lang="ts">
  import { onMount } from "svelte";
  import { api } from "./lib/api";
  import { app } from "./lib/state.svelte";
  import { gib, uptime } from "./lib/format";
  import Header from "./components/Header.svelte";
  import Gauge from "./components/Gauge.svelte";
  import Controls from "./components/Controls.svelte";
  import DevPanel from "./components/DevPanel.svelte";
  import DiskRow from "./components/DiskRow.svelte";
  import DiskView from "./components/DiskView.svelte";
  import SettingsView from "./components/SettingsView.svelte";
  import Icon from "./components/Icon.svelte";

  let root = $state<HTMLElement>();

  onMount(() => {
    app.init();
    // Pause the spinners whenever the WebView is hidden.
    const onVis = () => document.documentElement.classList.toggle("paused", document.hidden);
    document.addEventListener("visibilitychange", onVis);
    return () => document.removeEventListener("visibilitychange", onVis);
  });

  // Theme: always resolve to light/dark so app.css needs one block per theme;
  // "system" follows the OS live.
  $effect(() => {
    const t = app.settings?.theme ?? "system";
    const mq = matchMedia("(prefers-color-scheme: dark)");
    const apply = () =>
      (document.documentElement.dataset.theme = t === "system" ? (mq.matches ? "dark" : "light") : t);
    apply();
    if (t !== "system") return;
    mq.addEventListener("change", apply);
    return () => mq.removeEventListener("change", apply);
  });

  // Size the native window to the content so the flyout never scrolls.
  $effect(() => {
    if (!root) return;
    const ro = new ResizeObserver(([e]) => api.fitHeight(Math.ceil(e.borderBoxSize[0].blockSize)));
    ro.observe(root);
    return () => ro.disconnect();
  });

  const m = $derived(app.metrics);
  const memPct = $derived(m && m.memory.total ? (m.memory.used / m.memory.total) * 100 : 0);
  // What WSL costs Windows (vmmemWSL), which includes Linux's page cache.
  const memSub = $derived.by(() => {
    if (!m) return "";
    const parts = [];
    if (m.hostMemory) parts.push(`Windows ${gib(m.hostMemory)} GB`);
    if (m.memory.swapUsed) parts.push(`swap ${gib(m.memory.swapUsed)} GB`);
    return parts.join(" · ");
  });
  const version = $derived(
    app.selected ? [app.selected.default ? "Default" : "", `WSL ${app.selected.version}`].filter(Boolean).join(" · ") : "",
  );
</script>

<main bind:this={root}>
  {#if app.view === "settings"}
    <SettingsView />
  {:else if app.view === "disk"}
    <DiskView />
  {:else}
    <Header />

    <div class="body">
      {#if app.snapshot?.error}
        <div class="banner error">{app.snapshot.error}</div>
      {/if}
      {#if app.actionError}
        <button class="banner error" onclick={() => (app.actionError = "")} title="Dismiss">{app.actionError}</button>
      {:else if app.notice}
        <button class="banner info" onclick={() => (app.notice = "")} title="Dismiss">{app.notice}</button>
      {/if}

      <div class="meta">
        {#if app.snapshot?.running && m}
          <span>Up {uptime(m.uptimeSec)}</span><span class="dot">·</span>
        {/if}
        <span>{version}</span>
      </div>

      {#if app.snapshot?.running}
        <div class="gauges">
          <Gauge label="CPU" percent={m?.cpuPercent ?? 0} history={app.cpuHistory} />
          <Gauge
            label="Memory"
            percent={memPct}
            value={m ? `${gib(m.memory.used)} / ${gib(m.memory.total)} GB` : "–"}
            sub={memSub}
            title="Used inside Linux. 'Windows' is what the WSL VM takes from your PC, including Linux's file cache."
          >
            {#snippet action()}
              <button class="mini" onclick={() => app.freeMemory()} disabled={app.freeing}
                title="Release Linux's file cache so Windows gets the memory back">
                {app.freeing ? "Freeing…" : "Free"}
              </button>
            {/snippet}
          </Gauge>
        </div>
        <DiskRow />
        <Controls />
        <DevPanel />
      {:else if app.selected}
        <div class="stopped">
          <p class="title">{app.selected.name} is {app.busy ? app.busy + "…" : "stopped"}</p>
          <p class="hint">PenguinPeek won't start it on its own. Metrics appear once it is running.</p>
        </div>
        <Controls />
      {:else if app.snapshot && !app.snapshot.error}
        <div class="stopped"><p class="title">No WSL distributions found</p></div>
      {/if}
    </div>

    <footer>
      <span class="version">PenguinPeek {app.version}</span>
      <button class="link" onclick={() => api.refresh()} title="Refresh now" aria-label="Refresh now">
        <Icon name="refresh" size={14} />
      </button>
    </footer>
  {/if}
</main>

<style>
  main {
    display: flex;
    flex-direction: column;
  }
  .body {
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 0 12px 12px;
  }
  .meta {
    display: flex;
    gap: 6px;
    padding-left: 2px;
    margin: -2px 0 0 34px;
    color: var(--text-3);
    font-size: 12px;
    min-height: 17px;
  }
  .gauges {
    display: grid;
    grid-template-columns: 1fr 1.25fr;
    gap: 8px;
  }
  .stopped {
    padding: 18px 14px;
    text-align: center;
    background: var(--card);
    border: 1px solid var(--stroke);
    border-radius: var(--radius);
    box-shadow: var(--shadow);
  }
  .stopped .title {
    margin: 0;
    font: 600 14px var(--font-display);
  }
  .stopped .hint {
    margin: 4px 0 0;
    color: var(--text-3);
    font-size: 12px;
  }
  .mini {
    height: 20px;
    padding: 0 7px;
    border: 1px solid var(--stroke-strong);
    border-radius: 4px;
    background: transparent;
    color: var(--text-2);
    font-size: 11px;
  }
  .mini:hover:not(:disabled) {
    background: var(--card-hover);
    color: var(--text);
  }
  .mini:disabled {
    opacity: 0.6;
  }
  .banner.info {
    border-color: color-mix(in srgb, var(--accent) 40%, transparent);
    background: var(--accent-soft);
  }
  .banner {
    display: block;
    width: 100%;
    text-align: left;
    padding: 8px 10px;
    border-radius: 6px;
    font-size: 12px;
    border: 1px solid color-mix(in srgb, var(--high) 40%, transparent);
    background: color-mix(in srgb, var(--high) 12%, transparent);
  }
  footer {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 6px 8px;
    border-top: 1px solid var(--stroke);
    background: color-mix(in srgb, var(--card) 50%, var(--bg));
  }
  .link {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    height: 28px;
    padding: 0 8px;
    border: 0;
    border-radius: 6px;
    background: transparent;
    color: var(--text-2);
  }
  .link:hover:not(:disabled) {
    background: var(--card-hover);
    color: var(--text);
  }
  .version {
    padding: 0 6px;
    color: var(--text-3);
    font-size: 11.5px;
  }
</style>
