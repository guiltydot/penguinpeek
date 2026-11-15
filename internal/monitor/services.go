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
}

// DefaultWatched are always shown, running or not.
var DefaultWatched = []string{"docker", "postgres", "redis"}

// detectServices returns watched services plus any other detected running
// ones, in catalog order. procs are process names outside containers;
// inContainer maps containerized process names to their container ID, and
// names maps known container IDs to container names.
func detectServices(procs map[string]bool, inContainer, names map[string]string, distros []wsl.Distro, watched []string) []ServiceStatus {
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
		if def.ID == "docker" && !st.Running && desktop {
			st.Running, st.Detail, st.Controllable = true, "Docker Desktop", false
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
