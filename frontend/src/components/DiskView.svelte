<script lang="ts">
  import { app } from "../lib/state.svelte";
  import { gib } from "../lib/format";
  import Icon from "./Icon.svelte";

  const disk = $derived(app.metrics?.disk);
  const running = $derived(!!app.snapshot?.running);
  const reclaimable = $derived(disk && disk.file > disk.used ? disk.file - disk.used : 0);

  let trimming = $state(false);
  let trimResult = $state("");
  let confirming = $state(false);
  let compactStarted = $state(false);

  async function trim() {
    trimming = true;
    trimResult = (await app.trimDisk()) ?? "";
    trimming = false;
  }

  async function compact() {
    compactStarted = await app.compactDisk();
    confirming = false;
  }
</script>

<div class="page">
  <header>
    <button class="back" aria-label="Back" onclick={() => app.open("main")}><Icon name="back" /></button>
    <h1>Disk space</h1>
  </header>

  {#if app.actionError}
    <button class="banner error" onclick={() => (app.actionError = "")} title="Dismiss">{app.actionError}</button>
  {/if}

  <div class="stats">
    <div><span class="k">Used inside Linux</span><span class="v">{disk?.used ? `${gib(disk.used)} GB` : "–"}</span></div>
    <div><span class="k">Disk file on Windows</span><span class="v">{disk?.file ? `${gib(disk.file)} GB` : "–"}</span></div>
    <div><span class="k">Could be reclaimed</span><span class="v accent">{disk?.file ? `≈ ${gib(reclaimable)} GB` : "–"}</span></div>
  </div>
  {#if disk?.path}<p class="path" title={disk.path}>{disk.path}</p>{/if}

  <p class="intro">
    WSL's disk file grows as Linux writes but never shrinks on its own. Two steps give the space back to Windows.
  </p>

  <ol>
    <li>
      <div class="step">
        <div>
          <h2>Trim free space</h2>
          <p>Marks blocks Linux no longer uses as free. Safe, takes seconds.</p>
        </div>
        <button class="btn" onclick={trim} disabled={!running || trimming}>
          {#if trimming}<span class="spin"></span>{/if}Trim
        </button>
      </div>
      {#if trimResult}<p class="result">{trimResult}</p>{/if}
      {#if !running}<p class="result muted">Start the distribution to trim.</p>{/if}
    </li>
    <li>
      <div class="step">
        <div>
          <h2>Compact the disk file</h2>
          <p>Shuts down all of WSL, then shrinks the file. Windows asks for administrator approval.</p>
        </div>
        {#if !confirming}
          <button class="btn" onclick={() => (confirming = true)} disabled={!disk?.path}>Compact…</button>
        {/if}
      </div>
      {#if confirming}
        <div class="confirm">
          <p>Every running distribution and open WSL terminal will be closed. Save your work first.</p>
          <div class="row">
            <button class="btn" onclick={() => (confirming = false)}>Cancel</button>
            <button class="btn danger" onclick={compact}>Shut down WSL and compact</button>
          </div>
        </div>
      {/if}
      {#if compactStarted}
        <p class="result">Compaction is running in the administrator window. Sizes update when you reopen this page.</p>
      {/if}
    </li>
  </ol>
</div>

<style>
  .page {
    padding: 8px 12px 14px;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  header {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  h1 {
    margin: 0;
    font: 600 15px var(--font-display);
  }
  .back {
    display: grid;
    place-items: center;
    width: 30px;
    height: 30px;
    border: 0;
    border-radius: 6px;
    background: transparent;
  }
  .back:hover {
    background: var(--card-hover);
  }
  .stats {
    background: var(--card);
    border: 1px solid var(--stroke);
    border-radius: var(--radius);
    padding: 4px 12px;
    box-shadow: var(--shadow);
  }
  .stats div {
    display: flex;
    justify-content: space-between;
    padding: 5px 0;
  }
  .stats div + div {
    border-top: 1px solid var(--stroke);
  }
  .k {
    color: var(--text-2);
  }
  .v {
    font-weight: 600;
    font-variant-numeric: tabular-nums;
  }
  .accent {
    color: var(--accent);
  }
  .path {
    margin: -4px 2px 0;
    color: var(--text-3);
    font-size: 11px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .intro {
    margin: 0 2px;
    color: var(--text-2);
    font-size: 12px;
  }
  ol {
    margin: 0;
    padding: 0;
    list-style: none;
    counter-reset: step;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  li {
    counter-increment: step;
    background: var(--card);
    border: 1px solid var(--stroke);
    border-radius: var(--radius);
    padding: 10px 12px;
    box-shadow: var(--shadow);
  }
  .step {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .step > div {
    flex: 1;
    min-width: 0;
  }
  h2 {
    margin: 0;
    font: 600 13px var(--font);
  }
  h2::before {
    content: counter(step) ". ";
    color: var(--text-3);
  }
  .step p {
    margin: 2px 0 0;
    color: var(--text-3);
    font-size: 11.5px;
  }
  .btn {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    height: 30px;
    padding: 0 12px;
    flex: none;
    border-radius: 6px;
    border: 1px solid var(--stroke-strong);
    background: var(--card);
    white-space: nowrap;
  }
  .btn:hover:not(:disabled) {
    background: var(--card-hover);
  }
  .btn:disabled {
    opacity: 0.5;
  }
  .danger {
    background: var(--high);
    border-color: transparent;
    color: var(--danger-text);
    font-weight: 600;
  }
  .danger:hover:not(:disabled) {
    background: color-mix(in srgb, var(--high) 88%, #000);
  }
  .confirm {
    margin-top: 8px;
    padding: 8px 10px;
    border-radius: 6px;
    background: color-mix(in srgb, var(--high) 10%, transparent);
    border: 1px solid color-mix(in srgb, var(--high) 35%, transparent);
  }
  .confirm p {
    margin: 0 0 8px;
    font-size: 12px;
  }
  .row {
    display: flex;
    justify-content: flex-end;
    gap: 6px;
  }
  .result {
    margin: 6px 0 0;
    font-size: 12px;
    color: var(--ok);
  }
  .result.muted {
    color: var(--text-3);
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
</style>
