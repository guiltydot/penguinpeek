<script lang="ts">
  import { app } from "../lib/state.svelte";
  import Icon from "./Icon.svelte";
  import ServiceIcon from "./ServiceIcon.svelte";

  const themes = [
    { id: "system", label: "System" },
    { id: "light", label: "Light" },
    { id: "dark", label: "Dark" },
  ];
  const intervals = [2, 5, 10, 60];

  const alertOptions = [
    { key: "distroStopped", label: "A distribution stops unexpectedly" },
    { key: "serviceStopped", label: "A watched service stops" },
    { key: "highMemory", label: "WSL memory is above 90%" },
  ] as const;

  function setAlert(key: (typeof alertOptions)[number]["key"], on: boolean) {
    const cur = app.settings?.alerts ?? { distroStopped: true, serviceStopped: true, highMemory: true };
    app.saveSettings({ alerts: { ...cur, [key]: on } });
  }

  function toggleService(id: string, on: boolean) {
    const cur = app.settings?.watchedServices ?? [];
    app.saveSettings({ watchedServices: on ? [...cur, id] : cur.filter((x) => x !== id) });
  }
</script>

<div class="settings">
  <header>
    <button class="back" aria-label="Back" onclick={() => app.open("main")}><Icon name="back" /></button>
    <h1>Settings</h1>
  </header>

  {#if app.actionError}
    <button class="banner" onclick={() => (app.actionError = "")} title="Dismiss">{app.actionError}</button>
  {/if}

  {#if app.settings}
    {#if app.autostart !== null}
      <section>
        <label class="switch-row">
          <span>
            <span class="title">Start with Windows</span>
            <span class="hint inline">Opens in the tray when you sign in.</span>
          </span>
          <input type="checkbox" role="switch" class="switch" checked={app.autostart}
            onchange={(e) => app.setAutostart(e.currentTarget.checked)} />
        </label>
      </section>
    {/if}

    <section>
      <h2>Theme</h2>
      <div class="seg" role="radiogroup" aria-label="Theme">
        {#each themes as t (t.id)}
          <button role="radio" aria-checked={app.settings.theme === t.id}
            onclick={() => app.saveSettings({ theme: t.id })}>{t.label}</button>
        {/each}
      </div>
    </section>

    <section>
      <h2>Refresh while open</h2>
      <div class="seg" role="radiogroup" aria-label="Refresh interval">
        {#each intervals as s (s)}
          <button role="radio" aria-checked={app.settings.refreshSeconds === s}
            onclick={() => app.saveSettings({ refreshSeconds: s })}>{s < 60 ? ` s` : ` min`}</button>
        {/each}
      </div>
      <p class="hint">While the window is closed PenguinPeek checks every 30 s, and never starts a stopped distribution.</p>
    </section>

    <section>
      <h2>Always show these services</h2>
      <div class="list">
        {#each app.known as s (s.id)}
          <label>
            <input type="checkbox" checked={app.settings.watchedServices?.includes(s.id)}
              onchange={(e) => toggleService(s.id, e.currentTarget.checked)} />
            <ServiceIcon id={s.id} size={16} />
            {s.name}
          </label>
        {/each}
      </div>
      <p class="hint">Other known services appear automatically while they are running.</p>
    </section>

    <section>
      <h2>Notify me when</h2>
      <div class="list single">
        {#each alertOptions as a (a.key)}
          <label>
            <input type="checkbox" checked={app.settings.alerts?.[a.key] ?? true}
              onchange={(e) => setAlert(a.key, e.currentTarget.checked)} />
            {a.label}
          </label>
        {/each}
      </div>
      <p class="hint">
        Service and memory alerts check inside Linux every 30 s while the window is closed, only when the
        distribution is already running.
        <button class="link" onclick={() => app.testNotification()}>Send a test notification</button>
      </p>
      {#if app.notice}<p class="hint ok">{app.notice}</p>{/if}
    </section>
  {/if}
</div>

<style>
  .settings {
    padding: 8px 12px 14px;
    display: flex;
    flex-direction: column;
    gap: 14px;
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
  h2 {
    margin: 0 0 6px 2px;
    font: 600 12px var(--font);
    color: var(--text-2);
  }
  .seg {
    display: flex;
    padding: 3px;
    gap: 3px;
    background: var(--card);
    border: 1px solid var(--stroke);
    border-radius: var(--radius);
    box-shadow: var(--shadow);
  }
  .seg button {
    flex: 1;
    height: 28px;
    border: 0;
    border-radius: 5px;
    background: transparent;
    color: var(--text-2);
  }
  .seg button:hover {
    background: var(--card-hover);
  }
  .seg button[aria-checked="true"] {
    background: var(--accent-soft);
    color: var(--text);
    font-weight: 600;
    box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--accent) 45%, transparent);
  }
  .list {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 2px 8px;
    padding: 8px 10px;
    background: var(--card);
    border: 1px solid var(--stroke);
    border-radius: var(--radius);
    box-shadow: var(--shadow);
  }
  label {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 4px 0;
  }
  input {
    accent-color: var(--accent);
    width: 15px;
    height: 15px;
    margin: 0;
  }
  .hint {
    margin: 6px 2px 0;
    color: var(--text-3);
    font-size: 11.5px;
  }
  .link {
    padding: 0;
    border: 0;
    background: none;
    color: var(--accent);
    font-size: inherit;
    text-decoration: underline;
    text-underline-offset: 2px;
  }
  .hint.ok {
    color: var(--ok);
  }
  .hint.inline {
    display: block;
    margin: 1px 0 0;
  }
  .list.single {
    grid-template-columns: 1fr;
  }
  .switch-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 10px 12px;
    background: var(--card);
    border: 1px solid var(--stroke);
    border-radius: var(--radius);
    box-shadow: var(--shadow);
  }
  .title {
    font-weight: 600;
  }
  /* Windows 11 style toggle switch */
  .switch {
    appearance: none;
    width: 40px;
    height: 20px;
    flex: none;
    border-radius: 10px;
    border: 1px solid var(--text-2);
    background: transparent;
    position: relative;
  }
  .switch::after {
    content: "";
    position: absolute;
    top: 3px;
    left: 3px;
    width: 12px;
    height: 12px;
    border-radius: 50%;
    background: var(--text-2);
  }
  .switch:checked {
    background: var(--accent);
    border-color: var(--accent);
  }
  .switch:checked::after {
    translate: 20px 0;
    background: var(--accent-text);
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
</style>
