<script lang="ts">
  import type { Snippet } from "svelte";
  import { level } from "../lib/format";

  interface Props {
    label: string;
    percent: number;
    value?: string;
    sub?: string;
    /** Recent percentages; when given, a sparkline replaces the value line. */
    history?: number[];
    /** Optional small control shown next to the label. */
    action?: Snippet;
    /** Tooltip for the whole card. */
    title?: string;
  }
  let { label, percent, value = "", sub = "", history, action, title }: Props = $props();

  const SW = 120;
  const SH = 26;
  const spark = $derived.by(() => {
    if (!history || history.length < 2) return null;
    const step = SW / 29;
    const x0 = SW - (history.length - 1) * step;
    // Fixed steps (25/50/100 %) so low load stays visible without the
    // misleading wobble of a fully auto-scaled axis.
    const peak = Math.max(...history);
    const top = peak <= 25 ? 25 : peak <= 50 ? 50 : 100;
    const pts = history.map((v, i) => [x0 + i * step, SH - 1 - (Math.min(top, v) / top) * (SH - 2)]);
    const line = pts.map(([x, y]) => `${x.toFixed(1)},${y.toFixed(1)}`).join(" ");
    return { line, area: `${x0},${SH} ${line} ${SW},${SH}` };
  });

  const R = 30;
  const C = 2 * Math.PI * R;
  const pct = $derived(Math.max(0, Math.min(100, percent)));
  const dash = $derived(`${(pct / 100) * C} ${C}`);
  const sev = $derived(level(pct));
</script>

<div class="gauge" data-level={sev} {title}>
  <svg viewBox="0 0 72 72" aria-hidden="true">
    <circle class="track" cx="36" cy="36" r={R} />
    <circle class="bar" cx="36" cy="36" r={R} stroke-dasharray={dash} />
  </svg>
  <div class="center">
    <span class="pct">{Math.round(pct)}<small>%</small></span>
  </div>
  <div class="text">
    <div class="head">
      <span class="label">{label}</span>
      {@render action?.()}
    </div>
    {#if history}
      <svg class="spark" viewBox="0 0 {SW} {SH}" preserveAspectRatio="none" aria-hidden="true">
        <line x1="0" x2={SW} y1={SH - 0.5} y2={SH - 0.5} />
        {#if spark}
          <polygon points={spark.area} />
          <polyline points={spark.line} />
        {/if}
      </svg>
    {:else}
      <span class="value">{value}</span>
    {/if}
    {#if sub}<span class="sub">{sub}</span>{/if}
  </div>
</div>

<style>
  .gauge {
    display: grid;
    grid-template-columns: 56px 1fr;
    align-items: center;
    gap: 10px;
    padding: 12px;
    background: var(--card);
    border: 1px solid var(--stroke);
    border-radius: var(--radius);
    box-shadow: var(--shadow);
    position: relative;
    min-width: 0;
  }
  svg:not(.spark) {
    width: 56px;
    height: 56px;
    transform: rotate(-90deg);
    grid-row: 1;
    grid-column: 1;
  }
  circle {
    fill: none;
    stroke-width: 6;
  }
  .track {
    stroke: var(--track);
  }
  .bar {
    stroke: var(--accent);
    stroke-linecap: round;
  }
  [data-level="warn"] .bar {
    stroke: var(--warn);
  }
  [data-level="high"] .bar {
    stroke: var(--high);
  }
  .center {
    grid-row: 1;
    grid-column: 1;
    display: grid;
    place-items: center;
  }
  .pct {
    font: 600 14px var(--font-display);
    font-variant-numeric: tabular-nums;
  }
  .pct small {
    font-size: 9px;
    color: var(--text-2);
    margin-left: 1px;
  }
  .text {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 4px;
    min-height: 18px;
  }
  .label {
    color: var(--text-2);
    font-size: 12px;
  }
  .value {
    font: 600 15px var(--font-display);
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }
  .spark {
    width: 100%;
    height: 26px;
    margin: 2px 0 1px;
    transform: none;
    overflow: visible;
  }
  .spark line {
    stroke: var(--track);
    stroke-width: 1;
  }
  .spark polyline {
    fill: none;
    stroke: var(--accent);
    stroke-width: 1.5;
    stroke-linejoin: round;
    vector-effect: non-scaling-stroke;
  }
  .spark polygon {
    fill: var(--accent-soft);
    stroke: none;
  }
  [data-level="warn"] .spark polyline {
    stroke: var(--warn);
  }
  [data-level="high"] .spark polyline {
    stroke: var(--high);
  }
  .sub {
    color: var(--text-3);
    font-size: 11px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
</style>
