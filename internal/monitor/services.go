package monitor

import (
	"slices"

	"penguinpeek/internal/wsl"
)

// ServiceStatus is one development service as seen from the guest.
type ServiceStatus struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Running bool   `json:"running"`
	Detail  string `json:"detail"`
	// Controllable is false when the service runs outside the distro
	// (Docker Desktop) or only inside a container, so PenguinPeek cannot
	// start or stop it as a system service.
	Controllable bool `json:"controllable"`
	// InContainer is true when the service's process was found only inside
	// a Docker container (e.g. the nginx bundled in a web app image).
	InContainer bool `json:"inContainer"`
	// Container names that container, "" while it is not known yet.
	Container string `json:"container"`
}

// ServiceDef describes a detectable, controllable dev service.
type ServiceDef struct {
	ID, Name string
	Procs    []string // /proc/*/comm names (truncated to 15 chars by the kernel)
	Units    []string // systemd unit / init.d names to try, in order
}

// Catalog lists the services PenguinPeek knows how to detect.
var Catalog = []ServiceDef{
	{"docker", "Docker", []string{"dockerd"}, []string{"docker"}},
	{"postgres", "PostgreSQL", []string{"postgres"}, []string{"postgresql"}},
	{"redis", "Redis", []string{"redis-server"}, []string{"redis-server", "redis"}},
	{"mysql", "MySQL / MariaDB", []string{"mysqld", "mariadbd"}, []string{"mysql", "mariadb"}},
	{"mongodb", "MongoDB", []string{"mongod"}, []string{"mongod"}},
	{"nginx", "Nginx", []string{"nginx"}, []string{"nginx"}},
	{"tailscale", "Tailscale", []string{"tailscaled"}, []string{"tailscaled"}},
	// Tailscale SSH is a feature of tailscaled, not a process or unit: it is
	// up when tailscaled runs with `tailscale set --ssh`.
	{"tailscale-ssh", "Tailscale SSH", nil, nil},
	// OpenSSH 9.8+ names per-connection processes sshd-session.
	{"ssh", "OpenSSH", []string{"sshd", "sshd-session"}, []string{"ssh", "sshd"}},
}

// DefaultWatched are always shown, running or not.
var DefaultWatched = []string{"docker", "postgres", "redis"}

// guestState is what detectServices reads from one guest sample.
type guestState struct {
	procs map[string]bool // process names outside containers
	// inContainer maps containerized process names to their container ID,
	// and names maps known container IDs to container names.
	inContainer, names map[string]string
	ports              []wsl.Port
	tailscaleSSH       bool
}

// detectServices returns watched services plus any other detected running
// ones, in catalog order.
func detectServices(g guestState, distros []wsl.Distro, watched []string) []ServiceStatus {
	procs, inContainer, names := g.procs, g.inContainer, g.names
	desktop := false
	for _, d := range distros {
		if d.Name == "docker-desktop" && d.Running {
			desktop = true
		}
	}
	var out []ServiceStatus
	for _, def := range Catalog {
		st := ServiceStatus{ID: def.ID, Name: def.Name, Controllable: true}
		st.Running = slices.ContainsFunc(def.Procs, func(p string) bool { return procs[p] })
		if !st.Running {
			for _, p := range def.Procs {
				if id, ok := inContainer[p]; ok {
					st.Running, st.Controllable, st.InContainer = true, false, true
					st.Container = names[id]
					break
				}
			}
		}
		switch {
		case def.ID == "docker" && !st.Running && desktop:
			st.Running, st.Detail, st.Controllable = true, "Docker Desktop", false
		case def.ID == "tailscale-ssh":
			st.Running = g.tailscaleSSH && procs["tailscaled"]
			st.Detail, st.Controllable = "via tailscaled", false
		case def.ID == "ssh" && !st.Running && socketSSH(g.ports):
			// Socket activation (Ubuntu 22.10+): systemd holds port 22 and
			// starts sshd per connection, so no sshd runs while idle.
			st.Running, st.Detail = true, "socket-activated"
		}
		if st.Running || slices.Contains(watched, def.ID) {
			out = append(out, st)
		}
	}
	return out
}

// unnamedContainer reports whether a service runs in a container whose
// name is not known yet.
func unnamedContainer(ss []ServiceStatus) bool {
	return slices.ContainsFunc(ss, func(s ServiceStatus) bool { return s.InContainer && s.Container == "" })
}

// socketSSH reports whether something other than the user's own processes
// listens on port 22 on a non-loopback address, as systemd's ssh.socket does.
func socketSSH(ports []wsl.Port) bool {
	return slices.ContainsFunc(ports, func(p wsl.Port) bool {
		return p.Port == 22 && !p.Loopback && (p.Process == "" || p.Process == "systemd")
	})
}
