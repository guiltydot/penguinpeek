<script lang="ts">
  // A small coloured badge per dev service, greyed out while it is stopped.
  // Like DistroIcon these are simplified marks, not the official logos.
  let { id, size = 18, dim = false }: { id: string; size?: number; dim?: boolean } = $props();

  const COLOR: Record<string, string> = {
    docker: "#2496ED",
    postgres: "#336791",
    redis: "#D82C20",
    mysql: "#00758F",
    mongodb: "#13AA52",
    nginx: "#009639",
    tailscale: "#3B4252",
    "tailscale-ssh": "#3B4252",
    ssh: "#B45309",
    copilot: "#24292F",
  };
  const bg = $derived(COLOR[id] ?? "#64748B");
</script>

<svg class="badge" class:dim width={size} height={size} viewBox="0 0 20 20" aria-hidden="true">
  <rect width="20" height="20" rx="5" fill={bg} />
  <g fill="none" stroke="#fff" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
    {#if id === "docker"}
      <g fill="#fff" stroke="none">
        <rect x="4" y="8.6" width="2.6" height="2.4" rx=".4" /><rect x="7.2" y="8.6" width="2.6" height="2.4" rx=".4" />
        <rect x="10.4" y="8.6" width="2.6" height="2.4" rx=".4" /><rect x="7.2" y="5.6" width="2.6" height="2.4" rx=".4" />
        <rect x="10.4" y="5.6" width="2.6" height="2.4" rx=".4" />
      </g>
      <path d="M3.2 12c1.4 3.4 7.4 4 10.6.6 1-.9 1.6-1.8 2.9-2" />
    {:else if id === "postgres"}
      <!-- elephant head with a curled trunk -->
      <path d="M6 14.5V8.6a4 4 0 0 1 8 0V11c0 1.5-.9 2.4-2.1 2.4H11v2.3" />
      <circle cx="11.6" cy="8.4" r=".9" fill="#fff" stroke="none" />
    {:else if id === "redis"}
      <path d="M10 4.4 15.8 7 10 9.6 4.2 7z" fill="#fff" stroke="none" />
      <path d="m4.2 10.2 5.8 2.6 5.8-2.6M4.2 13.2l5.8 2.6 5.8-2.6" />
    {:else if id === "mysql"}
      <ellipse cx="10" cy="5.9" rx="5" ry="1.9" />
      <path d="M5 5.9v8.2c0 1 2.2 1.9 5 1.9s5-.9 5-1.9V5.9M5 10c0 1 2.2 1.9 5 1.9s5-.9 5-1.9" />
    {:else if id === "mongodb"}
      <path d="M10 3.4c3.1 3 3.6 6.7 0 11.1-3.6-4.4-3.1-8.1 0-11.1z" fill="#fff" stroke="none" />
      <path d="M10 14.2v2.4" />
    {:else if id === "nginx"}
      <path d="M10 3.4 15.7 6.7v6.6L10 16.6l-5.7-3.3V6.7z" />
      <path d="M8.1 12.6V7.4l3.8 5.2V7.4" />
    {:else if id === "tailscale"}
      <!-- a small mesh of connected nodes -->
      <path d="M6 6.2 14 8.4 7.6 14.2z" stroke-width="1.1" opacity=".7" />
      <g fill="#fff" stroke="none">
        <circle cx="6" cy="6.2" r="1.9" /><circle cx="14" cy="8.4" r="1.9" /><circle cx="7.6" cy="14.2" r="1.9" />
      </g>
    {:else if id === "tailscale-ssh"}
      <path d="m5.2 6.6 3.3 3.2-3.3 3.2M10.3 13.6h4.5" />
    {:else if id === "ssh"}
      <!-- a key -->
      <circle cx="7.2" cy="10" r="2.7" />
      <path d="M9.9 10h6.3M14.2 10v2.6M16.2 10v2" />
    {:else if id === "copilot"}
      <!-- a friendly face with goggles -->
      <path d="M4.2 12.4v-2.6a5.8 4.6 0 0 1 11.6 0v2.6c0 2.2-2.6 3.4-5.8 3.4s-5.8-1.2-5.8-3.4z" />
      <rect x="5.4" y="6.4" width="4" height="3.2" rx="1.4" />
      <rect x="10.6" y="6.4" width="4" height="3.2" rx="1.4" />
      <path d="M8.4 12v1.2M11.6 12v1.2" />
    {:else}
      <circle cx="10" cy="10" r="4.6" /><circle cx="10" cy="10" r="1.4" fill="#fff" stroke="none" />
    {/if}
  </g>
</svg>

<style>
  .badge {
    flex: none;
    display: block;
    border-radius: 5px;
    box-shadow: 0 1px 2px rgba(0, 0, 0, 0.2);
  }
  /* Stopped: grey instead of the brand colour. */
  .dim {
    filter: grayscale(1);
    opacity: 0.5;
    box-shadow: none;
  }
</style>
