package wsl

import "testing"

func TestParseContainers(t *testing.T) {
	out := `{"Command":"\"nginx\"","ID":"aaaaaaaaaaaa","Image":"nginx:1.27","Labels":"com.docker.compose.config-files=/a.yml,/b.yml,com.docker.compose.project=shop,com.docker.compose.service=web","Names":"shop-web-1","Ports":"0.0.0.0:8080->80/tcp","State":"running","Status":"Up 3 hours"}
not json
{"ID":"cccccccccccc","Image":"redis","Labels":"","Names":"solo","State":"exited","Status":"Exited (0) 2 days ago"}
{"ID":"bbbbbbbbbbbb","Image":"postgres:16","Labels":"com.docker.compose.project=shop,com.docker.compose.service=db","Names":"shop-db-1","State":"Exited","Status":"Exited (1) 1 minute ago"}
`
	cs := parseContainers(out)
	if len(cs) != 3 {
		t.Fatalf("got %d containers: %+v", len(cs), cs)
	}
	if cs[0].Name != "shop-db-1" || cs[0].Project != "shop" || cs[0].Service != "db" || cs[0].State != "exited" || cs[0].Running() {
		t.Errorf("first %+v", cs[0])
	}
	if cs[1].Name != "shop-web-1" || cs[1].Service != "web" || !cs[1].Running() || cs[1].Ports != "0.0.0.0:8080->80/tcp" {
		t.Errorf("second %+v", cs[1])
	}
	if cs[2].Name != "solo" || cs[2].Project != "" {
		t.Errorf("standalone should sort last: %+v", cs[2])
	}
}

func TestValidContainerAction(t *testing.T) {
	ok := []string{"aaaaaaaaaaaa"}
	if err := validContainerAction("stop", ok); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		action string
		ids    []string
	}{
		{"rm", ok},
		{"stop", nil},
		{"stop", []string{"--all"}},
		{"stop", []string{"web; rm -rf /"}},
		{"stop", []string{"AAAAAAAAAAAA"}},
	} {
		if validContainerAction(tc.action, tc.ids) == nil {
			t.Errorf("accepted %q %q", tc.action, tc.ids)
		}
	}
}
