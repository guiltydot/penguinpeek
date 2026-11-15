<script lang="ts">
  import { app } from "../lib/state.svelte";
  import Icon from "./Icon.svelte";
  import DistroPicker from "./DistroPicker.svelte";

  const status = $derived.by(() => {
    if (app.busy) return { kind: "busy", text: app.busy[0].toUpperCase() + app.busy.slice(1) + "…" };
    if (app.snapshot?.running) return { kind: "on", text: "Running" };
    if (app.selected) return { kind: "off", text: "Stopped" };
    return { kind: "off", text: "Unavailable" };
  });
</script>

<header>
  <DistroPicker />
  <span class="pill" data-kind={status.kind}><i></i>{status.text}</span>
  <button class="icon-btn" title="Settings" aria-label="Settings" onclick={() => app.open("settings")}>
    <Icon name="settings" />
  </button>
</header>

<style>
  header {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 12px 4px 10px;
  }
  .pill {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    flex: none;
    padding: 2px 9px 2px 8px;
    border-radius: 999px;
    background: var(--card);
    border: 1px solid var(--stroke);
    font-size: 12px;
    color: var(--text-2);
    box-shadow: var(--shadow);
  }
  .pill i {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--idle-dot);
  }
  .pill[data-kind="on"] i {
    background: var(--ok-dot);
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--ok-dot) 22%, transparent);
  }
  .pill[data-kind="busy"] i {
    background: var(--busy-dot);
  }
  .icon-btn {
    display: grid;
    place-items: center;
    width: 30px;
    height: 30px;
    flex: none;
    border: 0;
    border-radius: 6px;
    background: transparent;
    color: var(--text-2);
  }
  .icon-btn:hover {
    background: var(--card-hover);
    color: var(--text);
  }
</style>
