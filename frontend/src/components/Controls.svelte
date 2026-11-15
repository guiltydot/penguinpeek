<script lang="ts">
  import { app } from "../lib/state.svelte";
  import Icon from "./Icon.svelte";

  const running = $derived(!!app.snapshot?.running);
  const label = $derived(app.busy ? app.busy[0].toUpperCase() + app.busy.slice(1) + "…" : "");
  const none = $derived(!app.selected);
</script>

<div class="controls">
  {#if app.busy}
    <button class="btn busy" disabled aria-live="polite"><span class="spin"></span>{label}</button>
  {:else if running}
    <button class="btn" onclick={app.stop} disabled={none}><Icon name="stop" size={14} />Stop</button>
    <button class="btn" onclick={app.restart} disabled={none}><Icon name="restart" size={14} />Restart</button>
  {:else}
    <button class="btn primary" onclick={app.start} disabled={none}><Icon name="play" size={14} />Start</button>
  {/if}
  <div class="quick" role="group" aria-label="Open">
    <button class="icon" onclick={app.terminal} disabled={none} title="Terminal" aria-label="Open terminal">
      <Icon name="terminal" />
    </button>
    <button class="icon" onclick={app.files} disabled={none} title="Home folder in File Explorer" aria-label="Open home folder">
      <Icon name="folder" />
    </button>
    <button class="icon" onclick={app.vscode} disabled={none} title="VS Code (WSL)" aria-label="Open in VS Code">
      <Icon name="code" />
    </button>
  </div>
</div>

<style>
  .controls {
    display: flex;
    gap: 6px;
  }
  .btn,
  .icon {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    height: 32px;
    border-radius: 6px;
    border: 1px solid var(--stroke-strong);
    background: var(--card);
    box-shadow: var(--shadow);
  }
  .btn {
    flex: 1;
    gap: 6px;
    padding: 0 10px;
    min-width: 0;
  }
  .quick {
    display: flex;
    gap: 4px;
  }
  .icon {
    width: 34px;
    color: var(--text-2);
  }
  .btn:hover:not(:disabled),
  .icon:hover:not(:disabled) {
    background: var(--card-hover);
    color: var(--text);
  }
  .btn:active:not(:disabled) {
    color: var(--text-2);
  }
  .btn:disabled,
  .icon:disabled {
    opacity: 0.5;
  }
  .btn.busy {
    opacity: 1;
    color: var(--text-2);
  }
  .primary {
    background: var(--accent);
    color: var(--accent-text);
    border-color: transparent;
    font-weight: 600;
  }
  .primary:hover:not(:disabled) {
    background: color-mix(in srgb, var(--accent) 90%, var(--bg));
    color: var(--accent-text);
  }
  .spin {
    width: 14px;
    height: 14px;
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
