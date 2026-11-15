<script lang="ts">
  // Small inline icon set (Fluent-like strokes) to avoid an icon dependency.
  type Name =
    | "play" | "stop" | "restart" | "terminal" | "settings" | "back" | "chevron" | "refresh"
    | "folder" | "code" | "external" | "copy" | "disk" | "sweep" | "box";
  let { name, size = 16 }: { name: Name; size?: number } = $props();
</script>

<svg width={size} height={size} viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.3"
  stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
  {#if name === "play"}
    <path d="M5 3.5v9l7-4.5z" fill="currentColor" stroke="none" />
  {:else if name === "stop"}
    <rect x="4" y="4" width="8" height="8" rx="1.5" fill="currentColor" stroke="none" />
  {:else if name === "restart"}
    <path d="M13 8a5 5 0 1 1-1.5-3.55" /><path d="M13 2.5v2.5h-2.5" />
  {:else if name === "terminal"}
    <rect x="1.5" y="2.5" width="13" height="11" rx="2" /><path d="M4.5 6.5 6.5 8l-2 1.5M8.5 10h3" />
  {:else if name === "settings"}
    <circle cx="8" cy="8" r="2.1" />
    <path d="M6.9 1.8h2.2l.35 1.6 1.2.7 1.55-.5 1.1 1.9-1.2 1.1v1.4l1.2 1.1-1.1 1.9-1.55-.5-1.2.7-.35 1.6H6.9l-.35-1.6-1.2-.7-1.55.5-1.1-1.9 1.2-1.1V7.3l-1.2-1.1 1.1-1.9 1.55.5 1.2-.7z" />
  {:else if name === "back"}
    <path d="M10 3 5 8l5 5" />
  {:else if name === "chevron"}
    <path d="m4 6 4 4 4-4" />
  {:else if name === "refresh"}
    <path d="M2.5 8a5.5 5.5 0 0 1 9.4-3.9L13.5 5.5M13.5 2.5v3h-3M13.5 8a5.5 5.5 0 0 1-9.4 3.9L2.5 10.5M2.5 13.5v-3h3" />
  {:else if name === "folder"}
    <path d="M1.8 4.2c0-.7.5-1.2 1.2-1.2h3l1.4 1.5H13c.7 0 1.2.5 1.2 1.2v6.1c0 .7-.5 1.2-1.2 1.2H3c-.7 0-1.2-.5-1.2-1.2z" />
  {:else if name === "code"}
    <path d="M5.5 4.5 2 8l3.5 3.5M10.5 4.5 14 8l-3.5 3.5M9 3l-2 10" />
  {:else if name === "external"}
    <path d="M9 2.5h4.5V7M13.5 2.5 7.5 8.5M12 9.5v3c0 .6-.4 1-1 1H3.5c-.6 0-1-.4-1-1V5c0-.6.4-1 1-1h3" />
  {:else if name === "copy"}
    <rect x="5" y="5" width="8.5" height="8.5" rx="1.5" /><path d="M3.5 10.5h-.5c-.6 0-1-.4-1-1V3c0-.6.4-1 1-1h6.5c.6 0 1 .4 1 1v.5" />
  {:else if name === "disk"}
    <rect x="2" y="3.5" width="12" height="9" rx="2" /><path d="M2 9.5h12" /><circle cx="11.5" cy="11" r=".5" fill="currentColor" />
  {:else if name === "sweep"}
    <path d="M9.5 2 7 7.5M4 8h6.5l1 5.5h-8.5z" /><path d="M6.5 10.5v3M9 10.5v3" />
  {:else if name === "box"}
    <path d="M8 1.8 13.8 5v6L8 14.2 2.2 11V5z" /><path d="M2.2 5 8 8.2 13.8 5M8 8.2v6" />
  {/if}
</svg>
