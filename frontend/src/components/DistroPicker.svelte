<script lang="ts">
  import { api } from "../lib/api";
  import { app } from "../lib/state.svelte";
  import DistroIcon from "./DistroIcon.svelte";
  import Icon from "./Icon.svelte";

  // Dev preview: ?picker opens the menu.
  let open = $state(import.meta.env.DEV && new URLSearchParams(location.search).has("picker"));
  let root = $state<HTMLElement>();

  function choose(name: string) {
    open = false;
    if (name !== app.snapshot?.selected) app.select(name);
  }

  // The installed distros are cached in settings; this re-reads them after
  // installing or removing one (a newly installed one also appears on its own
  // once it runs).
  function rescan() {
    open = false;
    api.refreshDistros();
  }

  // Close on outside click or Escape while open.
  $effect(() => {
    if (!open) return;
    const onDown = (e: PointerEvent) => {
      if (root && !root.contains(e.target as Node)) open = false;
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.stopPropagation();
        open = false;
      }
    };
    window.addEventListener("pointerdown", onDown, true);
    window.addEventListener("keydown", onKey, true);
    return () => {
      window.removeEventListener("pointerdown", onDown, true);
      window.removeEventListener("keydown", onKey, true);
    };
  });
</script>

<div class="picker" bind:this={root}>
  <button
    class="current"
    onclick={() => (open = !open)}
    disabled={app.distros.length === 0}
    aria-haspopup="listbox"
    aria-expanded={open}
    title={app.selected ? `${app.selected.name} – switch distribution` : "No distributions"}
  >
    {#if app.selected}
      <DistroIcon kind={app.kindOf(app.selected.name)} size={22} />
      <span class="name">{app.selected.name}</span>
    {:else}
      <span class="name muted">No distribution</span>
    {/if}
    {#if app.distros.length > 0}<span class="chev" class:up={open}><Icon name="chevron" size={12} /></span>{/if}
  </button>

  {#if open}
    <ul class="menu" role="listbox" aria-label="Distributions">
      {#each app.distros as d (d.name)}
        <li>
          <button role="option" aria-selected={d.name === app.snapshot?.selected} onclick={() => choose(d.name)}>
            <DistroIcon kind={app.kindOf(d.name)} dim={!d.running} />
            <span class="name">{d.name}</span>
            {#if d.default}<span class="tag">default</span>{/if}
            <span class="state" class:on={d.running}>{d.running ? "Running" : "Stopped"}</span>
          </button>
        </li>
      {/each}
      <li role="none" class="foot">
        <button onclick={rescan} title="Look for installed or removed distributions">
          <Icon name="refresh" size={13} />
          <span>Refresh list</span>
        </button>
      </li>
    </ul>
  {/if}
</div>

<style>
  .picker {
    position: relative;
    min-width: 0;
    flex: 1;
  }
  .current {
    display: flex;
    align-items: center;
    gap: 8px;
    max-width: 100%;
    padding: 3px 8px 3px 3px;
    border: 1px solid transparent;
    border-radius: 8px;
    background: transparent;
  }
  .current:hover:not(:disabled) {
    background: var(--card-hover);
    border-color: var(--stroke);
  }
  .name {
    font: 600 14px var(--font-display);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .muted {
    color: var(--text-3);
  }
  .chev {
    display: flex;
    color: var(--text-2);
  }
  .chev.up {
    rotate: 180deg;
  }
  .menu {
    position: absolute;
    z-index: 10;
    top: calc(100% + 4px);
    left: 0;
    width: 300px;
    /* The native window is sized to the page, not the menu: scroll instead
       of spilling past its bottom edge. */
    max-height: calc(100vh - 56px);
    overflow-y: auto;
    margin: 0;
    padding: 4px;
    list-style: none;
    background: var(--solid);
    border: 1px solid var(--stroke-strong);
    border-radius: 10px;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.28);
  }
  .menu button {
    display: flex;
    align-items: center;
    gap: 9px;
    width: 100%;
    padding: 6px 8px;
    border: 0;
    border-radius: 6px;
    background: transparent;
    text-align: left;
  }
  .menu button:hover {
    background: var(--card-hover);
  }
  .menu button[aria-selected="true"] {
    background: var(--accent-soft);
  }
  .menu .name {
    font: 500 13px var(--font);
  }
  .tag {
    font-size: 10.5px;
    padding: 0 5px;
    border-radius: 4px;
    background: var(--accent-soft);
    color: var(--accent);
  }
  .state {
    margin-left: auto;
    font-size: 11.5px;
    color: var(--text-3);
  }
  .state.on {
    color: var(--ok);
  }
  .foot {
    margin-top: 4px;
    padding-top: 4px;
    border-top: 1px solid var(--stroke);
  }
  .foot button {
    color: var(--text-2);
    font-size: 12px;
  }
</style>
