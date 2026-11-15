package alerts

import (
	"testing"
	"time"

	"penguinpeek/internal/monitor"
	"penguinpeek/internal/wsl"
)

var all = Config{DistroStopped: true, ServiceStopped: true, HighMemory: true}

func snap(running bool, t time.Time, used uint64, svcRunning bool) monitor.Snapshot {
	s := monitor.Snapshot{
		Distros:  []wsl.Distro{{Name: "Ubuntu", Running: running}, {Name: "docker-desktop", Running: running}},
		Selected: "Ubuntu",
		Running:  running,
	}
	if running {
		s.Metrics = &monitor.Metrics{
			Memory:    wsl.Memory{Total: 100, Used: used},
			Services:  []monitor.ServiceStatus{{ID: "postgres", Name: "PostgreSQL", Running: svcRunning}},
			SampledAt: t,
		}
	}
	return s
}

func TestDistroStoppedUnexpectedly(t *testing.T) {
	var e Evaluator
	t0 := time.Now()
	if a := e.Check(snap(true, t0, 10, true), all, t0); len(a) != 0 {
		t.Fatalf("baseline alerted: %+v", a)
	}
	a := e.Check(snap(false, t0, 0, false), all, t0)
	if len(a) != 1 || a[0].ID != "distro-Ubuntu" {
		t.Fatalf("want one Ubuntu alert (docker-desktop ignored), got %+v", a)
	}
}

func TestQuietStopIsIgnored(t *testing.T) {
	var e Evaluator
	t0 := time.Now()
	e.Check(snap(true, t0, 10, true), all, t0)
	s := snap(false, t0, 0, false)
	s.Quiet = []string{"Ubuntu"}
	if a := e.Check(s, all, t0); len(a) != 0 {
		t.Fatalf("user stop alerted: %+v", a)
	}
}

func TestListFailureIsNotAStop(t *testing.T) {
	var e Evaluator
	t0 := time.Now()
	e.Check(snap(true, t0, 10, true), all, t0)
	s := monitor.Snapshot{Error: "WSL is not available"}
	if a := e.Check(s, all, t0); len(a) != 0 {
		t.Fatalf("error alerted: %+v", a)
	}
}

func TestServiceStopped(t *testing.T) {
	var e Evaluator
	t0 := time.Now()
	e.Check(snap(true, t0, 10, true), all, t0)
	a := e.Check(snap(true, t0.Add(time.Second), 10, false), all, t0)
	if len(a) != 1 || a[0].ID != "service-Ubuntu/postgres" {
		t.Fatalf("got %+v", a)
	}
	// Same sample again (no new data) must not repeat.
	if a := e.Check(snap(true, t0.Add(time.Second), 10, false), all, t0); len(a) != 0 {
		t.Fatalf("repeated: %+v", a)
	}
}

func TestMemoryHysteresisAndCooldown(t *testing.T) {
	var e Evaluator
	t0 := time.Now()
	e.Check(snap(true, t0, 50, true), all, t0)
	if a := e.Check(snap(true, t0.Add(1*time.Second), 95, true), all, t0); len(a) != 1 || a[0].ID != "memory" {
		t.Fatalf("want memory alert, got %+v", a)
	}
	if a := e.Check(snap(true, t0.Add(2*time.Second), 96, true), all, t0); len(a) != 0 {
		t.Fatalf("repeated while high: %+v", a)
	}
	e.Check(snap(true, t0.Add(3*time.Second), 50, true), all, t0) // re-arm
	if a := e.Check(snap(true, t0.Add(4*time.Second), 95, true), all, t0.Add(time.Minute)); len(a) != 0 {
		t.Fatalf("cooldown ignored: %+v", a)
	}
	// Still high once the cooldown has passed: remind once.
	if a := e.Check(snap(true, t0.Add(5*time.Second), 95, true), all, t0.Add(20*time.Minute)); len(a) != 1 {
		t.Fatalf("want reminder after cooldown, got %+v", a)
	}
}

func TestDisabledAlertsStaySilent(t *testing.T) {
	var e Evaluator
	t0 := time.Now()
	e.Check(snap(true, t0, 10, true), Config{}, t0)
	if a := e.Check(snap(false, t0, 0, false), Config{}, t0); len(a) != 0 {
		t.Fatalf("disabled alerted: %+v", a)
	}
}
