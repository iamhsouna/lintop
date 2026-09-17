package app

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const updateRepo = "iamhsouna/lintop"

type ghReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type ghRelease struct {
	TagName    string           `json:"tag_name"`
	Name       string           `json:"name"`
	HTMLURL    string           `json:"html_url"`
	Assets     []ghReleaseAsset `json:"assets"`
	Prerelease bool             `json:"prerelease"`
}

func httpGet(url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "lintop-updater")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 128<<20))
}

func fetchLatestRelease() (*ghRelease, error) {
	body, err := httpGet("https://api.github.com/repos/" + updateRepo + "/releases/latest")
	if err != nil {
		return nil, err
	}
	var rel ghRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return nil, err
	}
	if rel.TagName == "" {
		return nil, fmt.Errorf("no release tag found")
	}
	return &rel, nil
}

// pickReleaseAsset chooses the best asset for the current OS/architecture.
func pickReleaseAsset(assets []ghReleaseAsset) *ghReleaseAsset {
	archAliases := map[string][]string{
		"amd64": {"amd64", "x86_64"},
		"arm64": {"arm64", "aarch64"},
	}
	aliases := archAliases[runtime.GOARCH]
	if len(aliases) == 0 {
		aliases = []string{runtime.GOARCH}
	}

	var fallback *ghReleaseAsset
	for i := range assets {
		name := strings.ToLower(assets[i].Name)
		if !strings.Contains(name, runtime.GOOS) {
			continue
		}
		archMatch := false
		for _, a := range aliases {
			if strings.Contains(name, a) {
				archMatch = true
				break
			}
		}
		if !archMatch {
			continue
		}
		if strings.Contains(name, "sha256") || strings.Contains(name, "checksums") ||
			strings.HasSuffix(name, ".sig") {
			continue
		}
		if strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tgz") {
			return &assets[i]
		}
		if fallback == nil && !strings.Contains(name, ".") {
			fallback = &assets[i]
		}
	}
	if fallback != nil {
		return fallback
	}
	// Last resort: any tar.gz archive.
	for i := range assets {
		name := strings.ToLower(assets[i].Name)
		if strings.HasSuffix(name, ".tar.gz") && !strings.Contains(name, "sha256") {
			return &assets[i]
		}
	}
	return nil
}

// extractBinaryFromArchive pulls the lintop binary out of a .tar.gz payload.
func extractBinaryFromArchive(data []byte) ([]byte, error) {
	gz, err := gzip.NewReader(strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if filepath.Base(hdr.Name) == "lintop" || filepath.Base(hdr.Name) == "mactop" {
			return io.ReadAll(io.LimitReader(tr, 256<<20))
		}
	}
	return nil, fmt.Errorf("lintop binary not found in archive")
}

func downloadReleaseBinary(asset *ghReleaseAsset) ([]byte, error) {
	body, err := httpGet(asset.BrowserDownloadURL)
	if err != nil {
		return nil, err
	}
	name := strings.ToLower(asset.Name)
	if strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tgz") {
		return extractBinaryFromArchive(body)
	}
	return body, nil
}

// runSelfUpdate checks GitHub for a newer lintop release and replaces the
// running executable in place. It is invoked by `lintop --update`.
func runSelfUpdate() {
	fmt.Printf("lintop %s — checking for updates...\n", version)

	rel, err := fetchLatestRelease()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Update check failed: %v\n", err)
		fmt.Fprintf(os.Stderr, "You can update manually:\n  curl -fsSL https://raw.githubusercontent.com/%s/main/install.sh | bash -s -- --update\n", updateRepo)
		os.Exit(1)
	}

	if versionsEqual(rel.TagName, version) {
		fmt.Printf("Already up to date (%s).\n", rel.TagName)
		return
	}

	asset := pickReleaseAsset(rel.Assets)
	if asset == nil {
		fmt.Fprintf(os.Stderr, "No prebuilt binary found for %s/%s in release %s.\n", runtime.GOOS, runtime.GOARCH, rel.TagName)
		fmt.Fprintf(os.Stderr, "Build from source instead: https://github.com/%s\n", updateRepo)
		os.Exit(1)
	}

	fmt.Printf("Updating %s -> %s (%s)\n", version, rel.TagName, asset.Name)
	data, err := downloadReleaseBinary(asset)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Download failed: %v\n", err)
		os.Exit(1)
	}
	if len(data) < 1024 {
		fmt.Fprintln(os.Stderr, "Downloaded file looks too small; aborting.")
		os.Exit(1)
	}

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot locate the running executable: %v\n", err)
		os.Exit(1)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	dir := filepath.Dir(exe)
	tmp, err := os.CreateTemp(dir, ".lintop-update-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot write next to %s: %v\n", exe, err)
		fmt.Fprintf(os.Stderr, "Try: sudo lintop --update\n")
		os.Exit(1)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		fmt.Fprintf(os.Stderr, "Failed to write update: %v\n", err)
		os.Exit(1)
	}
	if err := tmp.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to finalize update: %v\n", err)
		os.Exit(1)
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to set permissions: %v\n", err)
		os.Exit(1)
	}
	if err := os.Rename(tmpName, exe); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to replace %s: %v\n", exe, err)
		fmt.Fprintf(os.Stderr, "Try: sudo lintop --update\n")
		os.Exit(1)
	}

	fmt.Printf("Updated to %s.\n", rel.TagName)
}

// versionsEqual compares tags tolerating a leading "v" and a "-next"/build suffix.
func versionsEqual(a, b string) bool {
	clean := func(s string) string {
		s = strings.TrimSpace(s)
		s = strings.TrimPrefix(s, "v")
		if i := strings.IndexAny(s, "+-"); i >= 0 {
			s = s[:i]
		}
		return s
	}
	return clean(a) == clean(b)
}
