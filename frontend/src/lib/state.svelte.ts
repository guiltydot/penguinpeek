// App-wide reactive state built on Svelte 5 runes. Go pushes snapshots; this
// module only stores them and runs user actions. It never polls.
import { api, errorMessage, type ContainerVerb, type Settings, type Snapshot, type ServiceInfo } from "./api";
import { distroKind, type DistroKind } from "./distro";

/** Sparkline length: 30 samples is a minute at the default 2 s interval. */
const HISTORY = 30;
const TAB_KEY = "penguinpeek.tab";
const OS_KEY = "penguinpeek.os";

/** os-release IDs seen per distro, kept so stopped distros keep their icon. */
function loadOS(): Record<string, [string, string]> {
  try {
    return JSON.parse(localStorage.getItem(OS_KEY) ?? "{}");
  } catch {
    return {};
  }
}

type View = "main" | "settings" | "disk";
type Tab = "services" | "containers" | "ports";
const TABS: Tab[] = ["services", "containers", "ports"];

function loadTab(): Tab {
  try {
    const t = localStorage.getItem(TAB_KEY) as Tab;
    return TABS.includes(t) ? t : "services";
  } catch {
    return "services";
  }
}

class AppState {
  snapshot = $state<Snapshot | null>(null);
  settings = $state<Settings | null>(null);
  known = $state<ServiceInfo[]>([]);
  version = $state("");
  /** Window has the Acrylic backdrop (Windows 11 22H2+). */
  view = $state<View>("main");
  tab = $state<Tab>(loadTab());
  /** Last action error, shown as a dismissable banner. */
  actionError = $state("");
  /** Short confirmation of a finished action (e.g. memory released). */
  notice = $state("");
  /** Optimistic busy label while waiting for the first pushed busy state. */
  pending = $state("");
  /** Recent CPU % samples for the sparkline, built from pushed snapshots only. */
  cpuHistory = $state<number[]>([]);
  /** Service IDs with a start/stop in flight. */
  serviceBusy = $state<Record<string, boolean>>({});
  /** Container IDs / "project:<name>" targets with an action in flight. */
  containerBusy = $state<Record<string, boolean>>({});
  freeing = $state(false);
  osInfo = $state<Record<string, [string, string]>>(loadOS());
  autostart = $state<boolean | null>(null);

  distros = $derived(this.snapshot?.distros ?? []);
  selected = $derived(this.distros.find((d) => d.name === this.snapshot?.selected) ?? null);
  busy = $derived(this.snapshot?.busy || this.pending);
  metrics = $derived(this.snapshot?.running ? (this.snapshot?.metrics ?? null) : null);

  private noticeTimer: ReturnType<typeof setTimeout> | undefined;

  async init() {
    api.onSnapshot((s) => {
      this.record(s);
      this.snapshot = s;
      if (s.busy) this.pending = "";
    });
    api.onSettings((s) => (this.settings = s));
    const [snap, settings, known, version] = await Promise.all([
      api.snapshot(),
      api.settings(),
      api.knownServices(),
      api.version(),
    ]);
    this.version = version;
    this.snapshot ??= snap;
    this.settings = settings;
    this.known = known;
    // Dev preview: ?view=settings|disk and ?tab=ports open those directly.
    if (import.meta.env.DEV) {
      const q = new URLSearchParams(location.search);
      const v = q.get("view");
      if (v === "settings" || v === "disk") this.open(v);
      const t = q.get("tab") as Tab;
      if (TABS.includes(t)) this.tab = t;
    }
    void api.setContainersWatch(this.tab === "containers");
  }

  /** Icon family for a distro: os-release when known, else its name. */
  kindOf(name: string): DistroKind {
    const os = this.osInfo[name];
    return distroKind(name, os?.[0], os?.[1]);
  }

  private rememberOS(s: Snapshot) {
    const m = s.metrics;
    if (!s.running || !m?.osId) return;
    const prev = this.osInfo[s.selected];
    if (prev?.[0] === m.osId && prev?.[1] === m.osLike) return;
    this.osInfo = { ...this.osInfo, [s.selected]: [m.osId, m.osLike] };
    try {
      localStorage.setItem(OS_KEY, JSON.stringify(this.osInfo));
    } catch {
      // not persisted; the name-based guess covers stopped distros
    }
  }

  private record(s: Snapshot) {
    this.rememberOS(s);
    if (s.selected !== this.snapshot?.selected || !s.running) this.cpuHistory = [];
    if (s.live && s.metrics) this.cpuHistory = [...this.cpuHistory, s.metrics.cpuPercent].slice(-HISTORY);
  }

  open(view: View) {
    this.view = view;
    this.actionError = "";
    if (view === "settings") void this.loadAutostart();
  }

  setTab(tab: Tab) {
    this.tab = tab;
    void api.setContainersWatch(tab === "containers");
    try {
      localStorage.setItem(TAB_KEY, tab);
    } catch {
      // storage unavailable: the tab just isn't remembered
    }
  }

  showNotice(text: string) {
    this.notice = text;
    clearTimeout(this.noticeTimer);
    this.noticeTimer = setTimeout(() => (this.notice = ""), 6000);
  }

  private async attempt<T>(f: () => Promise<T>): Promise<T | undefined> {
    this.actionError = "";
    try {
      return await f();
    } catch (err) {
      this.actionError = errorMessage(err);
      return undefined;
    }
  }

  private get name() {
    return this.snapshot?.selected ?? "";
  }

  async run(label: string, f: (name: string) => Promise<void>) {
    const name = this.name;
    if (!name || this.busy) return;
    this.pending = label;
    await this.attempt(() => f(name));
    this.pending = "";
  }

  start = () => this.run("starting", api.start);
  stop = () => this.run("stopping", api.stop);
  restart = () => this.run("restarting", api.restart);
  terminal = () => this.name && this.attempt(() => api.openTerminal(this.name));
  files = () => this.name && this.attempt(() => api.openFiles(this.name));
  vscode = () => this.name && this.attempt(() => api.openVSCode(this.name));

  async serviceControl(id: string, action: "start" | "stop") {
    const name = this.name;
    if (!name || this.serviceBusy[id]) return;
    this.serviceBusy = { ...this.serviceBusy, [id]: true };
    await this.attempt(() => api.serviceControl(name, id, action));
    const { [id]: _, ...rest } = this.serviceBusy;
    this.serviceBusy = rest;
  }

  async containerAction(target: string, action: ContainerVerb) {
    const name = this.name;
    if (!name || this.containerBusy[target]) return;
    this.containerBusy = { ...this.containerBusy, [target]: true };
    await this.attempt(() => api.containerAction(name, action, target));
    const { [target]: _, ...rest } = this.containerBusy;
    this.containerBusy = rest;
  }

  openPort = (port: number) => this.attempt(() => api.openPort(port));

  async copyAddress(port: number) {
    await this.attempt(() => api.copyText(`localhost:${port}`));
    if (!this.actionError) this.showNotice(`Copied localhost:${port}`);
  }

  async freeMemory() {
    const name = this.name;
    if (!name || this.freeing) return;
    this.freeing = true;
    await this.attempt(() => api.freeMemory(name));
    this.freeing = false;
    if (!this.actionError) this.showNotice("Linux cache released. Windows takes the memory back over the next minute.");
  }

  async trimDisk(): Promise<string | undefined> {
    const name = this.name;
    if (!name) return undefined;
    return this.attempt(() => api.trimDisk(name));
  }

  async compactDisk(): Promise<boolean> {
    const name = this.name;
    if (!name) return false;
    await this.attempt(() => api.compactDisk(name));
    return !this.actionError;
  }

  async select(name: string) {
    await this.attempt(() => api.select(name));
  }

  async testNotification() {
    await this.attempt(() => api.testNotification());
    if (!this.actionError) this.showNotice("Test notification sent.");
  }

  async loadAutostart() {
    try {
      this.autostart = await api.autostart();
    } catch {
      this.autostart = null; // unsupported (browser preview without Windows)
    }
  }

  async setAutostart(on: boolean) {
    const prev = this.autostart;
    this.autostart = on;
    await this.attempt(() => api.setAutostart(on));
    if (this.actionError) this.autostart = prev;
  }

  async saveSettings(patch: Partial<Settings>) {
    if (!this.settings) return;
    const next = { ...this.settings, ...patch };
    this.settings = next; // optimistic; Go echoes the normalized value back
    const saved = await this.attempt(() => api.saveSettings(next));
    if (saved) this.settings = saved;
  }
}

export const app = new AppState();
