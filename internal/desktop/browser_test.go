package desktop

import "testing"

func TestShouldOpenBrowser(t *testing.T) {
	tests := map[string]bool{
		"":      true,
		"true":  true,
		"1":     true,
		"yes":   true,
		"false": false,
		"0":     false,
		"no":    false,
	}

	for value, want := range tests {
		if got := ShouldOpenBrowser(value); got != want {
			t.Fatalf("ShouldOpenBrowser(%q) = %v, want %v", value, got, want)
		}
	}
}

func TestOpenBrowserCommand(t *testing.T) {
	tests := []struct {
		goos string
		name string
	}{
		{goos: "windows", name: "rundll32"},
		{goos: "darwin", name: "open"},
		{goos: "linux", name: "xdg-open"},
	}

	for _, tt := range tests {
		name, args, err := openBrowserCommand(tt.goos, "http://localhost:8080")
		if err != nil {
			t.Fatalf("openBrowserCommand(%q) error = %v", tt.goos, err)
		}
		if name != tt.name {
			t.Fatalf("openBrowserCommand(%q) command = %q, want %q", tt.goos, name, tt.name)
		}
		if len(args) == 0 {
			t.Fatalf("openBrowserCommand(%q) args are empty", tt.goos)
		}
	}
}

func TestOpenBrowserCommandRejectsEmptyURL(t *testing.T) {
	if _, _, err := openBrowserCommand("linux", " "); err == nil {
		t.Fatal("expected empty URL error")
	}
}
