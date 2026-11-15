// The only module that talks to Go. Components use these functions and the
// reactive state in state.svelte.ts, never the generated bindings directly.
//
// Outside the Wails WebView (plain `pnpm dev` in a browser) a small mock
// backend is used so the UI can be developed and previewed without Windows.
import { Clipboard, Events } from "@wailsio/runtime";
import { PeekService } from "../../bindings/penguinpeek";
import * as EventNames from "../../bindings/events";
import type { Snapshot } from "../../bindings/penguinpeek/internal/monitor/models";
import type { Settings } from "../../bindings/penguinpeek/internal/settings/models";
import type { ServiceInfo } from "../../bindings/penguinpeek/models";
import type { Container } from "../../bindings/penguinpeek/internal/wsl/models";
import { mock } from "./mock";

export type { Snapshot, Settings, ServiceInfo, Container };
export type ContainerVerb = "start" | "stop" | "restart";

const inWails = !!(window as { chrome?: { webview?: unknown } }).chrome?.webview;

export interface Backend {
  version(): Promise<string>;
  snapshot(): Promise<Snapshot>;
  settings(): Promise<Settings>;
  saveSettings(s: Settings): Promise<Settings>;
  knownServices(): Promise<ServiceInfo[]>;
  select(name: string): Promise<void>;
  start(name: string): Promise<void>;
  stop(name: string): Promise<void>;
  restart(name: string): Promise<void>;
  openTerminal(name: string): Promise<void>;
  openFiles(name: string): Promise<void>;
  openVSCode(name: string): Promise<void>;
  serviceControl(name: string, id: string, action: "start" | "stop"): Promise<void>;
  setContainersWatch(on: boolean): Promise<void>;
  /** target is a container ID or "project:<name>". */
  containerAction(name: string, action: ContainerVerb, target: string): Promise<void>;
  openPort(port: number): Promise<void>;
  copyText(text: string): Promise<void>;
  freeMemory(name: string): Promise<void>;
  trimDisk(name: string): Promise<string>;
  compactDisk(name: string): Promise<void>;
  testNotification(): Promise<void>;
  autostart(): Promise<boolean>;
  setAutostart(on: boolean): Promise<void>;
  refresh(): Promise<void>;
  refreshDistros(): Promise<void>;
  fitHeight(px: number): Promise<void>;
  onSnapshot(cb: (s: Snapshot) => void): () => void;
  onSettings(cb: (s: Settings) => void): () => void;
}

const wails: Backend = {
  version: () => PeekService.Version(),
  snapshot: () => PeekService.Snapshot(),
  settings: () => PeekService.Settings(),
  saveSettings: (s) => PeekService.SaveSettings(s),
  knownServices: async () => (await PeekService.KnownServices()) ?? [],
  select: (n) => PeekService.Select(n),
  start: (n) => PeekService.Start(n),
  stop: (n) => PeekService.Stop(n),
  restart: (n) => PeekService.Restart(n),
  openTerminal: (n) => PeekService.OpenTerminal(n),
  openFiles: (n) => PeekService.OpenFiles(n),
  openVSCode: (n) => PeekService.OpenVSCode(n),
  serviceControl: (n, id, action) => PeekService.ServiceControl(n, id, action),
  setContainersWatch: (on) => PeekService.SetContainersWatch(on),
  containerAction: (n, action, target) => PeekService.ContainerAction(n, action, target),
  openPort: (p) => PeekService.OpenPort(p),
  copyText: async (t) => {
    await Clipboard.SetText(t);
  },
  freeMemory: (n) => PeekService.FreeMemory(n),
  trimDisk: (n) => PeekService.TrimDisk(n),
  compactDisk: (n) => PeekService.CompactDisk(n),
  testNotification: () => PeekService.TestNotification(),
  autostart: () => PeekService.Autostart(),
  setAutostart: (on) => PeekService.SetAutostart(on),
  refresh: () => PeekService.Refresh(),
  refreshDistros: () => PeekService.RefreshDistros(),
  fitHeight: (px) => PeekService.FitHeight(px),
  onSnapshot: (cb) => Events.On(EventNames.Snapshot, (e) => cb(e.data)),
  onSettings: (cb) => Events.On(EventNames.Settings, (e) => cb(e.data)),
};

export const api: Backend = inWails ? wails : mock;

/** Turns a rejected backend call into a short user-facing message. */
export function errorMessage(err: unknown): string {
  const raw = err instanceof Error ? err.message : String(err);
  // Wails wraps Go errors as JSON-ish text; keep only the message part.
  const m = /"message"\s*:\s*"([^"]+)"/.exec(raw);
  return m ? m[1] : raw;
}
