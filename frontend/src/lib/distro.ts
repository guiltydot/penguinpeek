// Works out which distribution family a WSL distro is, for its icon. A
// running distro reports /etc/os-release; a stopped one (which PenguinPeek
// never boots) is recognised by its registered name instead.

export type DistroKind =
  | "ubuntu" | "debian" | "fedora" | "arch" | "opensuse" | "kali" | "alpine"
  | "rhel" | "rocky" | "alma" | "oracle" | "docker" | "nixos" | "linux";

// Checked in order against os-release ID, then the name, then ID_LIKE.
const RULES: [RegExp, DistroKind][] = [
  [/docker/, "docker"],
  [/kali/, "kali"],
  [/ubuntu|pop|mint|elementary|zorin|neon/, "ubuntu"],
  [/debian|pengwin|raspbian/, "debian"],
  [/fedora|nobara/, "fedora"],
  [/arch|manjaro|endeavour|garuda/, "arch"],
  [/suse|sles/, "opensuse"],
  [/alpine/, "alpine"],
  [/rocky/, "rocky"],
  [/alma/, "alma"],
  [/oracle|\bol\b/, "oracle"],
  [/rhel|red ?hat|centos/, "rhel"],
  [/nixos/, "nixos"],
];

function match(text: string | undefined): DistroKind | null {
  if (!text) return null;
  const t = text.toLowerCase();
  for (const [re, kind] of RULES) if (re.test(t)) return kind;
  return null;
}

export function distroKind(name: string, osId?: string, osLike?: string): DistroKind {
  return match(osId) ?? match(name) ?? match(osLike) ?? "linux";
}

export const DISTRO_COLOR: Record<DistroKind, string> = {
  ubuntu: "#E95420",
  debian: "#C70036",
  fedora: "#3C6EB4",
  arch: "#1793D1",
  opensuse: "#73BA25",
  kali: "#367BF0",
  alpine: "#0D597F",
  rhel: "#EE0000",
  rocky: "#10B981",
  alma: "#0F4266",
  oracle: "#C74634",
  docker: "#2496ED",
  nixos: "#5277C3",
  linux: "#334155",
};
