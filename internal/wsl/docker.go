package wsl

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Container is one Docker container as `docker ps -a` reports it.
type Container struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Image   string `json:"image"`
	State   string `json:"state"`  // running, exited, paused, created, restarting, ...
	Status  string `json:"status"` // e.g. "Up 3 hours", "Exited (0) 2 days ago"
	Ports   string `json:"ports"`
	Project string `json:"project"` // compose project, "" when standalone
	Service string `json:"service"` // compose service
}

// Running reports whether the container is up (paused counts as up).
func (c Container) Running() bool {
	return c.State == "running" || c.State == "paused" || c.State == "restarting"
}

var (
	// ErrNoDockerCLI is returned when the distro has no `docker` command.
	ErrNoDockerCLI = errors.New("docker CLI not found")
	// ErrDockerUnavailable is returned when the CLI cannot reach a daemon.
	ErrDockerUnavailable = errors.New("docker daemon not reachable")
)

// dockerListScript lists all containers as JSON lines. Exit 3: no CLI,
// exit 4: the daemon did not answer.
const dockerListScript = `command -v docker >/dev/null 2>&1 || exit 3; docker ps -a --no-trunc --format '{{json .}}' || exit 4`

// dockerActionScript runs `docker <action> <ids...>`; values come as args.
const dockerActionScript = `a=$1; shift; exec docker "$a" "$@"`

var containerID = regexp.MustCompile(`^[0-9a-f]{12,64}$`)

// Containers lists every container visible to the distro's docker CLI. It
// runs as root so docker-group membership does not matter.
func (r *Runner) Containers(ctx context.Context, distro string) ([]Container, error) {
	out, code, err := r.rootExec(ctx, 10*time.Second, distro, dockerListScript)
	switch {
	case code == 3:
		return nil, ErrNoDockerCLI
	case code == 4:
		return nil, fmt.Errorf("%w: %v", ErrDockerUnavailable, err)
	case err != nil:
		return nil, err
	}
	return parseContainers(out), nil
}

// ContainerAction starts, stops or restarts containers by ID.
func (r *Runner) ContainerAction(ctx context.Context, distro, action string, ids []string) error {
	if err := validContainerAction(action, ids); err != nil {
		return err
	}
	_, _, err := r.rootExec(ctx, 90*time.Second, distro, dockerActionScript, append([]string{action}, ids...)...)
	if err != nil {
		return fmt.Errorf("docker %s: %w", action, err)
	}
	return nil
}

func validContainerAction(action string, ids []string) error {
	switch action {
	case "start", "stop", "restart":
	default:
		return fmt.Errorf("unsupported action %q", action)
	}
	if len(ids) == 0 {
		return errors.New("no containers given")
	}
	for _, id := range ids {
		if !containerID.MatchString(id) {
			return fmt.Errorf("invalid container id %q", id)
		}
	}
	return nil
}

// psLine is the subset of `docker ps --format '{{json .}}'` we use.
type psLine struct {
	ID     string `json:"ID"`
	Names  string `json:"Names"`
	Image  string `json:"Image"`
	State  string `json:"State"`
	Status string `json:"Status"`
	Ports  string `json:"Ports"`
	Labels string `json:"Labels"`
}

// parseContainers reads JSON lines, skipping anything unparsable, sorted by
// compose project (standalone last) and then name.
func parseContainers(out string) []Container {
	var cs []Container
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] != '{' {
			continue
		}
		var p psLine
		if json.Unmarshal([]byte(line), &p) != nil || p.ID == "" {
			continue
		}
		c := Container{
			ID:     p.ID,
			Name:   strings.TrimPrefix(strings.Split(p.Names, ",")[0], "/"),
			Image:  p.Image,
			State:  strings.ToLower(p.State),
			Status: p.Status,
			Ports:  p.Ports,
		}
		for kv := range strings.SplitSeq(p.Labels, ",") {
			k, v, _ := strings.Cut(kv, "=")
			switch k {
			case "com.docker.compose.project":
				c.Project = v
			case "com.docker.compose.service":
				c.Service = v
			}
		}
		cs = append(cs, c)
	}
	slices.SortFunc(cs, func(a, b Container) int {
		if (a.Project == "") != (b.Project == "") {
			if a.Project == "" {
				return 1
			}
			return -1
		}
		return cmp.Or(cmp.Compare(a.Project, b.Project), cmp.Compare(a.Name, b.Name))
	})
	return cs
}
