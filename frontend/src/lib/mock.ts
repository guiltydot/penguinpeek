// Browser-only stand-in for the Go backend, used by `pnpm dev` outside Wails.
// It mimics the push model: snapshots arrive on a timer, as events would.
import type { Backend, Snapshot, Settings } from "./api";
import pkg from "../../package.json";

const GiB = 1024 ** 3;
const subs = new Set<(s: Snapshot) => void>();
const settingsSubs = new Set<(s: Settings) => void>();

let settings: Settings = {
  distro: "",
  theme: "system",
  refreshSeconds: 60,
  watchedServices: ["docker", "postgres", "redis"],
  alerts: { distroStopped: true, serviceStopped: true, highMemory: true },
};
let autostart = false;

const state = {
  distros: [
    { name: "Ubuntu-24.04", default: true, version: 2, running: true },
    { name: "Debian", default: false, version: 2, running: false },
    { name: "kali-linux", default: false, version: 2, running: false },
    { name: "archlinux", default: false, version: 2, running: false },
    { name: "docker-desktop", default: false, version: 2, running: true },
  ],
  selected: "Ubuntu-24.04",
  busy: "",
  uptime: 4 * 3600 + 12 * 60,
  cpu: 7,
  services: { docker: true, postgres: true, redis: false, nginx: true, tailscale: true, "tailscale-ssh": true, ssh: false } as Record<string, boolean>,
  vhdx: 23.2 * GiB,
  containersWanted: false,
  containers: [
    { id: "a1b2c3d4e5f6", name: "shop-web-1", image: "node:22-alpine", state: "running", status: "Up 3 hours", ports: "0.0.0.0:3001->3000/tcp", project: "shop", service: "web" },
    { id: "b2c3d4e5f6a1", name: "shop-db-1", image: "postgres:16", state: "running", status: "Up 3 hours", ports: "5432/tcp", project: "shop", service: "db" },
    { id: "c3d4e5f6a1b2", name: "shop-cache-1", image: "redis:7", state: "exited", status: "Exited (0) 2 hours ago", ports: "", project: "shop", service: "cache" },
    { id: "d4e5f6a1b2c3", name: "mailpit", image: "axllent/mailpit", state: "exited", status: "Exited (137) 1 day ago", ports: "", project: "", service: "" },
  ],
};

function snap(): Snapshot {
  const sel = state.distros.find((d) => d.name === state.selected)!;
  state.cpu = Math.max(1, Math.min(96, state.cpu + (Math.random() - 0.45) * 12));
  const names: Record<string, string> = {
    docker: "Docker", postgres: "PostgreSQL", redis: "Redis", nginx: "Nginx",
    tailscale: "Tailscale", "tailscale-ssh": "Tailscale SSH", ssh: "OpenSSH",
  };
  return {
    distros: state.distros.map((d) => ({ ...d })),
    selected: state.selected,
    running: sel.running,
    busy: state.busy,
    error: "",
    live: sel.running,
    updatedAt: new Date().toISOString(),
    metrics: sel.running
      ? {
          cpuPercent: state.cpu,
          memory: { total: 7.7 * GiB, used: (1.6 + state.cpu / 60) * GiB, swapTotal: 2 * GiB, swapUsed: 0 },
          hostMemory: (2.1 + state.cpu / 50) * GiB,
          uptimeSec: (state.uptime += settings.refreshSeconds),
          sampledAt: new Date().toISOString(),
          home: "/home/dev",
          osId: "ubuntu",
          osLike: "debian",
          disk: { used: 14.6 * GiB, total: 1007 * GiB, file: state.vhdx, path: "C:\\WSL\\Ubuntu\\ext4.vhdx" },
          containers: state.containersWanted && state.services.docker ? state.containers.map((c) => ({ ...c })) : null,
          containersError: "",
          ports: [
            { port: 3000, process: "node", loopback: false },
            { port: 5173, process: "node", loopback: true },
            { port: 5432, process: "", loopback: false },
          ],
          services: Object.entries(state.services)
            .map(([id, running]) => ({
              id,
              name: names[id],
              running,
              detail: id === "docker" ? "Docker Desktop" : id === "tailscale-ssh" ? "via tailscaled" : "",
              controllable: id !== "docker" && id !== "nginx" && id !== "tailscale-ssh",
              // Preview a web app image's bundled nginx (e.g. BentoPDF).
              inContainer: id === "nginx",
              container: id === "nginx" ? "bentopdf" : "",
            }))
            .filter((s) => s.running || (settings.watchedServices ?? []).includes(s.id)),
        }
      : null,
  };
}

// Preview knobs for screenshots: ?distro=Debian&theme=light|dark&frame&gallery
const params = new URLSearchParams(location.search);
// ?gallery lists one distro per icon family.
if (params.has("gallery")) {
  for (const name of ["FedoraLinux-42", "openSUSE-Tumbleweed", "Alpine", "AlmaLinux-9", "OracleLinux_9_1", "NixOS", "MyCustomDistro"]) {
    state.distros.push({ name, default: false, version: 2, running: name === "Alpine" });
  }
}
if (params.get("distro")) state.selected = params.get("distro")!;
if (params.get("theme")) settings.theme = params.get("theme")!;
// Headless browsers enforce a minimum viewport; pin the flyout width instead.
if (params.has("frame")) document.documentElement.style.width = "360px";
const emit = () => subs.forEach((cb) => cb(snap()));
setInterval(emit, 2000);

const wait = (ms: number) => new Promise((r) => setTimeout(r, ms));

async function act(busy: string, name: string, running: boolean) {
  state.busy = busy;
  emit();
  await wait(1200);
  const d = state.distros.find((x) => x.name === name);
  if (d) d.running = running;
  if (running) state.uptime = 3;
  state.busy = "";
  emit();
}

export const mock: Backend = {
  version: async () => pkg.version,
  snapshot: async () => snap(),
  settings: async () => settings,
  saveSettings: async (s) => {
    settings = { ...s };
    settingsSubs.forEach((cb) => cb(settings));
    emit();
    return settings;
  },
  knownServices: async () => [
    { id: "docker", name: "Docker" },
    { id: "postgres", name: "PostgreSQL" },
    { id: "redis", name: "Redis" },
    { id: "mysql", name: "MySQL / MariaDB" },
    { id: "mongodb", name: "MongoDB" },
    { id: "nginx", name: "Nginx" },
    { id: "tailscale", name: "Tailscale" },
    { id: "tailscale-ssh", name: "Tailscale SSH" },
    { id: "ssh", name: "OpenSSH" },
  ],
  select: async (n) => {
    state.selected = n;
    emit();
  },
  start: (n) => act("starting", n, true),
  stop: (n) => act("stopping", n, false),
  restart: async (n) => {
    await act("restarting", n, true);
  },
  openTerminal: async () => {},
  openFiles: async () => {},
  openVSCode: async () => {},
  serviceControl: async (_n, id, action) => {
    await wait(900);
    state.services[id] = action === "start";
    emit();
  },
  setContainersWatch: async (on) => {
    state.containersWanted = on;
    emit();
  },
  containerAction: async (_n, action, target) => {
    await wait(900);
    const project = target.startsWith("project:") ? target.slice(8) : null;
    for (const c of state.containers) {
      if (project !== null ? c.project !== project : c.id !== target) continue;
      const up = action !== "stop";
      c.state = up ? "running" : "exited";
      c.status = up ? "Up 1 second" : "Exited (0) 1 second ago";
    }
    emit();
  },
  openPort: async (p) => {
    window.open(`http://localhost:${p}/`, "_blank");
  },
  copyText: async (t) => {
    await navigator.clipboard?.writeText(t);
  },
  freeMemory: async () => {
    await wait(700);
    emit();
  },
  trimDisk: async () => {
    await wait(1500);
    return "/: 3.1 GiB (3328599654 bytes) trimmed";
  },
  compactDisk: async () => {
    await wait(500);
  },
  testNotification: async () => {},
  autostart: async () => autostart,
  setAutostart: async (on) => {
    autostart = on;
  },
  refresh: async () => emit(),
  refreshDistros: async () => emit(),
  fitHeight: async () => {},
  onSnapshot: (cb) => {
    subs.add(cb);
    return () => subs.delete(cb);
  },
  onSettings: (cb) => {
    settingsSubs.add(cb);
    return () => settingsSubs.delete(cb);
  },
};
