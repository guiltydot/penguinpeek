<script lang="ts">
  import { app } from "../lib/state.svelte";
  import { gib } from "../lib/format";
  import Icon from "./Icon.svelte";

  const disk = $derived(app.metrics?.disk);
  // The VHDX only grows; space freed inside Linux stays allocated on Windows.
  const reclaimable = $derived(disk && disk.file > disk.used ? disk.file - disk.used : 0);
</script>

{#if disk && disk.used}
  <button class="disk" onclick={() => app.open("disk")} title="Disk space and shrinking">
    <Icon name="disk" />
    <span class="label">Disk</span>
    <span class="value">
      {gib(disk.used)} GB used{#if disk.file}<span class="sep">·</span>file {gib(disk.file)} GB{/if}
    </span>
    {#if reclaimable > 2 * 1024 ** 3}
      <span class="hint">{gib(reclaimable)} GB to reclaim</span>
    {/if}
    <span class="chev"><Icon name="chevron" size={12} /></span>
  </button>
{/if}

<style>
  .disk {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    height: 38px;
    padding: 0 10px 0 12px;
    text-align: left;
    background: var(--card);
    border: 1px solid var(--stroke);
    border-radius: var(--radius);
    box-shadow: var(--shadow);
    color: var(--text-2);
  }
  .disk:hover {
    background: var(--card-hover);
  }
  .label {
    color: var(--text-2);
    font-size: 12px;
  }
  .value {
    color: var(--text);
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }
  .sep {
    margin: 0 5px;
    color: var(--text-3);
  }
  .hint {
    margin-left: auto;
    font-size: 11px;
    color: var(--accent);
    white-space: nowrap;
  }
  .chev {
    display: flex;
    rotate: -90deg;
    color: var(--text-3);
  }
  .hint + .chev {
    margin-left: 0;
  }
  .value + .chev {
    margin-left: auto;
  }
</style>
