package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/registry"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	oras "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"

	"github.com/ForceCLI/force/sfplugin"
)

// startRegistry serves an in-memory registry over HTTP and returns its
// host.  wrap, when not nil, receives each request before the registry.
func startRegistry(t *testing.T, wrap func(next http.Handler) http.Handler) string {
	t.Helper()
	var handler http.Handler = registry.New(registry.Logger(log.New(io.Discard, "", 0)))
	if wrap != nil {
		handler = wrap(handler)
	}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	// No credentials are read from the Docker configuration of the
	// machine that runs the tests.
	t.Setenv("DOCKER_CONFIG", t.TempDir())
	return strings.TrimPrefix(server.URL, "http://")
}

func testClient() *Client {
	client := NewClient()
	client.PlainHTTP = true
	return client
}

func testManifest(version string) sfplugin.Manifest {
	return sfplugin.Manifest{
		Name:     "tool",
		Version:  version,
		Protocol: sfplugin.ProtocolVersion,
		Commands: []sfplugin.Command{{ID: "tool run", Summary: "Run the tool."}},
	}
}

func writeExecutable(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "built")
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func mustRef(t *testing.T, text string) Reference {
	t.Helper()
	ref, err := ParseReference(text)
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

func TestPublishThenInstallForThisPlatform(t *testing.T) {
	host := startRegistry(t, nil)
	ctx := context.Background()
	client := testClient()
	ref := mustRef(t, host+"/test/tool:v1")
	other := Platform{OS: "plan9", Arch: "mips"}
	result, err := client.Publish(ctx, ref, testManifest("1.0.0"), map[Platform]string{
		HostPlatform(): writeExecutable(t, "host executable"),
		other:          writeExecutable(t, "other executable"),
	}, PublishOptions{Source: "https://example.com/repo", Revision: "abc123"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Platforms) != 2 || result.IndexDigest == "" || result.Replaced != "" {
		t.Fatalf("publish result = %+v", result)
	}

	resolved, err := client.Resolve(ctx, ref, HostPlatform())
	if err != nil {
		t.Fatal(err)
	}
	if resolved.IndexDigest != result.IndexDigest || resolved.ManifestDigest != result.Platforms[HostPlatform().String()] {
		t.Errorf("resolved digests %s / %s do not match the publish result %+v", resolved.IndexDigest, resolved.ManifestDigest, result)
	}
	if resolved.Manifest.Version != "1.0.0" || resolved.FileName != ExecutableName("tool", HostPlatform()) {
		t.Errorf("resolved = %+v", resolved)
	}

	store := &Store{Dir: t.TempDir()}
	path, err := client.Fetch(ctx, resolved, store)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "host executable" {
		t.Errorf("fetched file = %q, %v", data, err)
	}
	if info, _ := os.Stat(path); info.Mode().Perm()&0o100 == 0 {
		t.Errorf("fetched file has mode %v, want it executable", info.Mode())
	}

	otherResolved, err := client.Resolve(ctx, ref, other)
	if err != nil {
		t.Fatal(err)
	}
	otherPath, err := client.Fetch(ctx, otherResolved, store)
	if err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(otherPath); string(data) != "other executable" {
		t.Errorf("fetched file for %s = %q", other, data)
	}
}

func TestResolveReportsThePublishedPlatforms(t *testing.T) {
	host := startRegistry(t, nil)
	ctx := context.Background()
	client := testClient()
	ref := mustRef(t, host+"/test/tool:v1")
	if _, err := client.Publish(ctx, ref, testManifest("1.0.0"), map[Platform]string{
		{OS: "linux", Arch: "amd64"}:  writeExecutable(t, "a"),
		{OS: "darwin", Arch: "arm64"}: writeExecutable(t, "b"),
	}, PublishOptions{}); err != nil {
		t.Fatal(err)
	}
	_, err := client.Resolve(ctx, ref, Platform{OS: "plan9", Arch: "mips"})
	if err == nil || !strings.Contains(err.Error(), "no executable for plan9/mips") || !strings.Contains(err.Error(), "darwin/arm64, linux/amd64") {
		t.Errorf("Resolve for a missing platform = %v", err)
	}
}

func TestResolveOfAMissingTag(t *testing.T) {
	host := startRegistry(t, nil)
	_, err := testClient().Resolve(context.Background(), mustRef(t, host+"/test/tool:v1"), HostPlatform())
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Resolve of a missing tag = %v, want ErrNotFound", err)
	}
}

func TestPublishToAnExistingTagNeedsOverwrite(t *testing.T) {
	host := startRegistry(t, nil)
	ctx := context.Background()
	client := testClient()
	ref := mustRef(t, host+"/test/tool:v1")
	executables := map[Platform]string{HostPlatform(): writeExecutable(t, "first")}
	first, err := client.Publish(ctx, ref, testManifest("1.0.0"), executables, PublishOptions{})
	if err != nil {
		t.Fatal(err)
	}
	executables[HostPlatform()] = writeExecutable(t, "second")
	if _, err := client.Publish(ctx, ref, testManifest("1.0.1"), executables, PublishOptions{}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("second publish without overwrite = %v", err)
	}
	second, err := client.Publish(ctx, ref, testManifest("1.0.1"), executables, PublishOptions{Overwrite: true})
	if err != nil {
		t.Fatal(err)
	}
	if second.Replaced != first.IndexDigest || second.IndexDigest == first.IndexDigest {
		t.Errorf("overwrite result = %+v, first digest %s", second, first.IndexDigest)
	}
	resolved, err := client.Resolve(ctx, ref, HostPlatform())
	if err != nil || resolved.Manifest.Version != "1.0.1" {
		t.Errorf("after overwrite: version %q, err %v", resolved.Manifest.Version, err)
	}
}

func TestFetchRejectsAnExecutableThatDoesNotMatchItsDigest(t *testing.T) {
	corrupt := false
	host := startRegistry(t, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !corrupt || r.Method != http.MethodGet || !strings.Contains(r.URL.Path, "/blobs/") {
				next.ServeHTTP(w, r)
				return
			}
			recorder := httptest.NewRecorder()
			next.ServeHTTP(recorder, r)
			body := recorder.Body.Bytes()
			if len(body) > 0 {
				body[0] ^= 0xff
			}
			for name, values := range recorder.Header() {
				w.Header()[name] = values
			}
			w.WriteHeader(recorder.Code)
			w.Write(body)
		})
	})
	ctx := context.Background()
	client := testClient()
	ref := mustRef(t, host+"/test/tool:v1")
	if _, err := client.Publish(ctx, ref, testManifest("1.0.0"), map[Platform]string{HostPlatform(): writeExecutable(t, "genuine bytes")}, PublishOptions{}); err != nil {
		t.Fatal(err)
	}
	resolved, err := client.Resolve(ctx, ref, HostPlatform())
	if err != nil {
		t.Fatal(err)
	}
	corrupt = true
	store := &Store{Dir: t.TempDir()}
	path, err := client.Fetch(ctx, resolved, store)
	if err == nil {
		t.Fatalf("Fetch of altered bytes succeeded and wrote %s", path)
	}
	dir, _ := store.digestDir(resolved.ManifestDigest)
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("files left in the store after a failed download: %v", entries)
	}
}

func TestResolveRejectsOtherArtifacts(t *testing.T) {
	host := startRegistry(t, nil)
	ctx := context.Background()
	client := testClient()
	ref := mustRef(t, host+"/test/other:v1")
	repo, err := client.repository(ref)
	if err != nil {
		t.Fatal(err)
	}

	// A tag that points to an image manifest instead of an index.
	blob := []byte("data")
	layer := content.NewDescriptorFromBytes("application/octet-stream", blob)
	if err := repo.Blobs().Push(ctx, layer, bytes.NewReader(blob)); err != nil {
		t.Fatal(err)
	}
	manifestDesc, err := oras.PackManifest(ctx, repo, oras.PackManifestVersion1_1, "application/vnd.example.other", oras.PackManifestOptions{Layers: []ocispec.Descriptor{layer}})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Tag(ctx, manifestDesc, "v1"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Resolve(ctx, ref, HostPlatform()); err == nil || !strings.Contains(err.Error(), "not an OCI image index") {
		t.Errorf("Resolve of an image manifest = %v", err)
	}

	// An index whose manifest for this platform has another artifact type.
	manifestDesc.Platform = &ocispec.Platform{OS: HostPlatform().OS, Architecture: HostPlatform().Arch}
	index := ocispec.Index{MediaType: ocispec.MediaTypeImageIndex, Manifests: []ocispec.Descriptor{manifestDesc}}
	index.SchemaVersion = 2
	indexData, _ := json.Marshal(index)
	indexDesc := content.NewDescriptorFromBytes(ocispec.MediaTypeImageIndex, indexData)
	if err := repo.Manifests().PushReference(ctx, indexDesc, bytes.NewReader(indexData), "v2"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Resolve(ctx, mustRef(t, host+"/test/other:v2"), HostPlatform()); err == nil || !strings.Contains(err.Error(), "is not an sf plugin") {
		t.Errorf("Resolve of another artifact type = %v", err)
	}
}

func TestFetchReusesAnExecutableAlreadyInTheStore(t *testing.T) {
	blobRequests := 0
	host := startRegistry(t, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/blobs/sha256:") {
				blobRequests++
			}
			next.ServeHTTP(w, r)
		})
	})
	ctx := context.Background()
	client := testClient()
	ref := mustRef(t, host+"/test/tool:v1")
	if _, err := client.Publish(ctx, ref, testManifest("1.0.0"), map[Platform]string{HostPlatform(): writeExecutable(t, "bytes")}, PublishOptions{}); err != nil {
		t.Fatal(err)
	}
	resolved, err := client.Resolve(ctx, ref, HostPlatform())
	if err != nil {
		t.Fatal(err)
	}
	store := &Store{Dir: t.TempDir()}
	if _, err := client.Fetch(ctx, resolved, store); err != nil {
		t.Fatal(err)
	}
	before := blobRequests
	if _, err := client.Fetch(ctx, resolved, store); err != nil {
		t.Fatal(err)
	}
	if blobRequests != before {
		t.Errorf("the second Fetch made %d blob requests, want 0", blobRequests-before)
	}
}
