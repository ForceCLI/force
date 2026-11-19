package sf

import (
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
)

// privateBrowsers maps the identifier of a default browser to the value of
// --browser it corresponds to.  sf, through the "open" package, supports a
// private window for these browsers only.
var privateBrowsers = map[string]string{
	"com.google.chrome":      "chrome",
	"google-chrome.desktop":  "chrome",
	"chromehtml":             "chrome",
	"com.brave.browser":      "brave",
	"brave-browser.desktop":  "brave",
	"bravehtml":              "brave",
	"org.mozilla.firefox":    "firefox",
	"firefox.desktop":        "firefox",
	"firefoxurl":             "firefox",
	"com.microsoft.msedge":   "edge",
	"com.microsoft.edge":     "edge",
	"com.microsoft.edgemac":  "edge",
	"microsoft-edge.desktop": "edge",
	"msedgehtm":              "edge",
}

var privateFlags = map[string]string{"chrome": "--incognito", "brave": "--incognito", "firefox": "--private-window", "edge": "--inPrivate"}

var windowsProgID = regexp.MustCompile(`(?i)ProgId\s+REG_SZ\s+(\S+)`)
var macHTTPHandler = regexp.MustCompile(`(?is)LSHandlerRoleAll\s*=\s*"?([^";\s]+)"?;[^}]*?LSHandlerURLScheme\s*=\s*https?;`)

// defaultBrowserID returns the identifier of the default browser: the
// desktop file on Linux, the bundle identifier on macOS and the ProgId on
// Windows.  Tests replace it.
var defaultBrowserID = func() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("defaults", "read", "com.apple.LaunchServices/com.apple.launchservices.secure", "LSHandlers").Output()
		if err != nil {
			return "", err
		}
		if match := macHTTPHandler.FindSubmatch(out); match != nil {
			return string(match[1]), nil
		}
		// Safari is the default when no handler is recorded.
		return "com.apple.safari", nil
	case "windows":
		out, err := exec.Command("reg", "query", `HKEY_CURRENT_USER\Software\Microsoft\Windows\Shell\Associations\UrlAssociations\http\UserChoice`, "/v", "ProgId").Output()
		if err != nil {
			return "", err
		}
		if match := windowsProgID.FindSubmatch(out); match != nil {
			return string(match[1]), nil
		}
		return "", fmt.Errorf("the default browser is not recorded in the registry")
	}
	out, err := exec.Command("xdg-settings", "get", "default-web-browser").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// privateBrowserCommand returns the command that opens a URL in a private
// window of the default browser.
func privateBrowserCommand(goos, id, uri string) (string, []string, error) {
	key := strings.ToLower(id)
	// A Windows ProgId can carry a suffix, as in FirefoxURL-308046B0AF4A39CB.
	if i := strings.IndexAny(key, "-."); goos == "windows" && i > 0 {
		key = key[:i]
	}
	browser, ok := privateBrowsers[key]
	if !ok {
		return "", nil, fmt.Errorf("%s is not supported as a default browser", id)
	}
	flag := privateFlags[browser]
	switch goos {
	case "darwin":
		app := map[string]string{"chrome": "google chrome", "brave": "brave browser", "firefox": "firefox", "edge": "microsoft edge"}[browser]
		return "open", []string{"-n", "-a", app, uri, "--args", flag}, nil
	case "windows":
		app := map[string]string{"chrome": "chrome", "brave": "brave", "firefox": "firefox", "edge": "msedge"}[browser]
		return "cmd", []string{"/c", "start", "", app, flag, uri}, nil
	}
	programs := browserCommands[browser]
	if browser == "brave" {
		programs = []string{"brave-browser", "brave"}
	}
	for _, name := range programs {
		if path, err := exec.LookPath(name); err == nil {
			return path, []string{flag, uri}, nil
		}
	}
	return "", nil, newError("BrowserNotFoundError", "Unable to find the %s browser. Tried: %s", browser, strings.Join(programs, ", "))
}

// openPrivately opens a URL in a private window of the default browser.
// Tests replace it.
var openPrivately = func(uri string) error {
	id, err := defaultBrowserID()
	if err != nil {
		return newError("DefaultBrowserError", "Unable to determine the default browser: %s", err.Error())
	}
	name, args, err := privateBrowserCommand(runtime.GOOS, id, uri)
	if err != nil {
		if failure, ok := err.(*Error); ok {
			return failure
		}
		return newError("Error", "%s", err.Error())
	}
	return exec.Command(name, args...).Start()
}
