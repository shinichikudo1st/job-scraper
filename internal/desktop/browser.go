package desktop

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

func ShouldOpenBrowser(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return true
	}
	return value == "1" || value == "true" || value == "yes"
}

func OpenBrowser(url string) error {
	name, args, err := openBrowserCommand(runtime.GOOS, url)
	if err != nil {
		return err
	}
	cmd := exec.Command(name, args...)
	return cmd.Start()
}

func openBrowserCommand(goos, url string) (string, []string, error) {
	url = strings.TrimSpace(url)
	if url == "" {
		return "", nil, fmt.Errorf("browser url is required")
	}
	switch goos {
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", url}, nil
	case "darwin":
		return "open", []string{url}, nil
	case "linux":
		return "xdg-open", []string{url}, nil
	default:
		return "", nil, fmt.Errorf("open browser is not supported on %s", goos)
	}
}
