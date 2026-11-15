const GiB = 1024 ** 3;

export function gib(bytes: number): string {
  const v = bytes / GiB;
  return v >= 10 ? v.toFixed(0) : v.toFixed(1);
}

export function uptime(sec: number): string {
  const s = Math.floor(sec);
  const d = Math.floor(s / 86400);
  const h = Math.floor((s % 86400) / 3600);
  const m = Math.floor((s % 3600) / 60);
  if (d > 0) return `${d}d ${h}h`;
  if (h > 0) return `${h}h ${m}m`;
  if (m > 0) return `${m}m`;
  return `${s}s`;
}

/** Severity bucket used to colour gauges. */
export function level(pct: number): "ok" | "warn" | "high" {
  if (pct >= 85) return "high";
  if (pct >= 65) return "warn";
  return "ok";
}
