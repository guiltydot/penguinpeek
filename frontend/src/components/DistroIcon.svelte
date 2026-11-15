<script lang="ts">
  // A small brand-coloured badge with a simplified white mark per distro
  // family. Deliberately simplified marks, not the official logos.
  import { DISTRO_COLOR, type DistroKind } from "../lib/distro";

  let { kind, size = 20, dim = false }: { kind: DistroKind; size?: number; dim?: boolean } = $props();
  const bg = $derived(DISTRO_COLOR[kind]);
  const letter: Partial<Record<DistroKind, string>> = { rhel: "R", rocky: "R", alma: "A", oracle: "O" };
</script>

<svg class="badge" class:dim width={size} height={size} viewBox="0 0 20 20" aria-hidden="true">
  <rect width="20" height="20" rx="5.5" fill={bg} />
  <g fill="none" stroke="#fff" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">
    {#if kind === "ubuntu"}
      <circle cx="10" cy="10" r="4.4" />
      <circle cx="15.2" cy="10" r="1.9" fill="#fff" stroke={bg} stroke-width="1.1" />
      <circle cx="7.4" cy="14.5" r="1.9" fill="#fff" stroke={bg} stroke-width="1.1" />
      <circle cx="7.4" cy="5.5" r="1.9" fill="#fff" stroke={bg} stroke-width="1.1" />
    {:else if kind === "debian"}
      <path d="M12.6 4.6c-3.7-1.2-7.6 1.4-7.8 5.3-.2 3.4 2.5 6 5.7 5.9 2.6-.1 4.6-2.2 4.4-4.7-.1-2-1.8-3.5-3.8-3.4-1.6.1-2.8 1.4-2.7 3 .1 1.1 1 2 2.2 2" />
    {:else if kind === "fedora"}
      <path d="M12.8 5h-1a2.4 2.4 0 0 0-2.4 2.4V15" /><path d="M6.8 9.7h5.4" />
    {:else if kind === "arch"}
      <path d="M10 3.6 15.8 16c-1.6-1.1-3-1.8-4.1-2.1.2-1.3-.6-2.4-1.7-2.4s-1.9 1.1-1.7 2.4c-1.1.3-2.5 1-4.1 2.1z" fill="#fff" stroke="none" />
    {:else if kind === "opensuse"}
      <path d="M4.3 12.3c.2-3.5 3-5.8 6.3-5.6 2.5.1 4.4 1.6 5 3.6-1.5-.6-2.7-.4-3.4.5" />
      <circle cx="13.7" cy="11.6" r="1.5" fill="#fff" stroke="none" />
    {:else if kind === "kali"}
      <path d="M6.5 4.5v11" /><path d="M14 4.5 7 10.2l7.2 5.3" />
    {:else if kind === "alpine"}
      <path d="M2.8 15.2 7.6 7.8l3 4.2 2-2.8 4.6 6z" fill="#fff" stroke="none" />
    {:else if kind === "docker"}
      <g fill="#fff" stroke="none">
        <rect x="4" y="8.6" width="2.6" height="2.4" rx=".4" /><rect x="7.2" y="8.6" width="2.6" height="2.4" rx=".4" />
        <rect x="10.4" y="8.6" width="2.6" height="2.4" rx=".4" /><rect x="7.2" y="5.6" width="2.6" height="2.4" rx=".4" />
        <rect x="10.4" y="5.6" width="2.6" height="2.4" rx=".4" />
      </g>
      <path d="M3.2 12c1.4 3.4 7.4 4 10.6.6 1-.9 1.6-1.8 2.9-2" />
    {:else if kind === "nixos"}
      <path d="m6 5 8 10M14 5 6 15M4 10h12" />
    {:else if letter[kind]}
      <text x="10" y="14.3" text-anchor="middle" fill="#fff" stroke="none"
        font-family="Segoe UI Variable Display, Segoe UI, sans-serif" font-weight="700" font-size="11">{letter[kind]}</text>
    {:else}
      <!-- generic Linux: a small penguin face -->
      <ellipse cx="10" cy="11.6" rx="4.6" ry="4.3" fill="#fff" stroke="none" />
      <circle cx="8.3" cy="10.2" r="1" fill={bg} stroke="none" /><circle cx="11.7" cy="10.2" r="1" fill={bg} stroke="none" />
      <path d="M8.8 12.6h2.4L10 14z" fill="#F59E0B" stroke="none" />
    {/if}
  </g>
</svg>

<style>
  .badge {
    flex: none;
    display: block;
    border-radius: 5.5px;
    box-shadow: 0 1px 2px rgba(0, 0, 0, 0.2);
  }
  .dim {
    filter: saturate(0.35);
    opacity: 0.75;
  }
</style>
