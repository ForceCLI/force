package plugins

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	oras "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/errdef"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
	"oras.land/oras-go/v2/registry/remote/retry"

	"github.com/ForceCLI/force/sfplugin"
)

// Media types of a published plugin.  A tag points to an OCI image index
// with one image manifest for each platform.  Each manifest has the plugin
// manifest as its config and the executable as its only layer.
const (
	ArtifactType        = "application/vnd.forcecli.sf.plugin.v1"
	ConfigMediaType     = "application/vnd.forcecli.sf.plugin.config.v1+json"
	ExecutableMediaType = "application/vnd.forcecli.sf.plugin.executable.v1"
)

// Platform is an operating system and architecture, with the names Go uses
// for GOOS and GOARCH.
type Platform struct {
	OS   string
	Arch string
}

func (p Platform) String() string {
	return p.OS + "/" + p.Arch
}

// HostPlatform returns the platform this program runs on.
func HostPlatform() Platform {
	return Platform{OS: runtime.GOOS, Arch: runtime.GOARCH}
}

// ParsePlatform reads "os/arch".
func ParsePlatform(text string) (Platform, error) {
	osName, arch, ok := strings.Cut(text, "/")
	if !ok || osName == "" || arch == "" || strings.Contains(arch, "/") {
		return Platform{}, fmt.Errorf("platform %q is not in the form os/arch, such as linux/amd64", text)
	}
	return Platform{OS: osName, Arch: arch}, nil
}

// ExecutableName returns the file name of the executable of a plugin on a
// platform.
func ExecutableName(pluginName string, platform Platform) string {
	if platform.OS == "windows" {
		return pluginName + ".exe"
	}
	return pluginName
}

// ErrNotFound is returned when a tag does not exist in the registry.
var ErrNotFound = errors.New("not found in the registry")

// Client reads and writes plugins in registries.
type Client struct {
	// PlainHTTP makes requests with HTTP instead of HTTPS, for a registry
	// on a local machine.
	PlainHTTP bool

	httpClient remote.Client
}

// NewClient returns a client that authenticates with the credentials of
// the Docker configuration (~/.docker/config.json and its credential
// helpers), which "docker login" writes.  Without stored credentials for a
// registry the requests are anonymous.
func NewClient() *Client {
	client := &auth.Client{Client: retry.DefaultClient, Cache: auth.NewCache()}
	if store, err := credentials.NewStoreFromDocker(credentials.StoreOptions{}); err == nil {
		client.Credential = credentials.Credential(store)
	}
	return &Client{httpClient: client}
}

func (c *Client) repository(ref Reference) (*remote.Repository, error) {
	repo, err := remote.NewRepository(ref.Name())
	if err != nil {
		return nil, err
	}
	repo.PlainHTTP = c.PlainHTTP
	if c.httpClient != nil {
		repo.Client = c.httpClient
	}
	return repo, nil
}

// Resolved is a published plugin for one platform.
type Resolved struct {
	Reference Reference
	Platform  Platform
	// IndexDigest is the digest the tag points to.
	IndexDigest string
	// ManifestDigest is the digest of the manifest for the platform.
	ManifestDigest string
	Manifest       sfplugin.Manifest
	// FileName is the name of the executable.
	FileName string

	layer ocispec.Descriptor
}

// Resolve reads what a tag points to and returns the plugin for platform.
func (c *Client) Resolve(ctx context.Context, ref Reference, platform Platform) (Resolved, error) {
	repo, err := c.repository(ref)
	if err != nil {
		return Resolved{}, err
	}
	indexDesc, index, err := c.fetchIndex(ctx, repo, ref)
	if err != nil {
		return Resolved{}, err
	}
	var available []string
	var selected *ocispec.Descriptor
	for i, entry := range index.Manifests {
		if entry.Platform == nil {
			continue
		}
		available = append(available, entry.Platform.OS+"/"+entry.Platform.Architecture)
		if entry.Platform.OS == platform.OS && entry.Platform.Architecture == platform.Arch {
			selected = &index.Manifests[i]
		}
	}
	if selected == nil {
		sort.Strings(available)
		return Resolved{}, fmt.Errorf("%s has no executable for %s; it is published for: %s", ref, platform, strings.Join(available, ", "))
	}
	if selected.MediaType != ocispec.MediaTypeImageManifest {
		return Resolved{}, fmt.Errorf("%s: the entry for %s has the media type %s, not an OCI image manifest", ref, platform, selected.MediaType)
	}
	data, err := content.FetchAll(ctx, repo.Manifests(), *selected)
	if err != nil {
		return Resolved{}, fmt.Errorf("%s: reading the manifest for %s: %w", ref, platform, err)
	}
	var manifest ocispec.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Resolved{}, fmt.Errorf("%s: the manifest for %s is not valid JSON: %w", ref, platform, err)
	}
	if manifest.ArtifactType != ArtifactType {
		return Resolved{}, fmt.Errorf("%s is not an sf plugin: its artifact type is %q, not %s", ref, manifest.ArtifactType, ArtifactType)
	}
	if manifest.Config.MediaType != ConfigMediaType {
		return Resolved{}, fmt.Errorf("%s is not an sf plugin: its config media type is %q, not %s", ref, manifest.Config.MediaType, ConfigMediaType)
	}
	if len(manifest.Layers) != 1 || manifest.Layers[0].MediaType != ExecutableMediaType {
		return Resolved{}, fmt.Errorf("%s: the manifest for %s must have exactly one layer of type %s", ref, platform, ExecutableMediaType)
	}
	config, err := content.FetchAll(ctx, repo.Blobs(), manifest.Config)
	if err != nil {
		return Resolved{}, fmt.Errorf("%s: reading the plugin manifest: %w", ref, err)
	}
	pluginManifest, err := sfplugin.ParseManifest(config)
	if err != nil {
		return Resolved{}, fmt.Errorf("%s: %w", ref, err)
	}
	fileName := manifest.Layers[0].Annotations[ocispec.AnnotationTitle]
	if fileName == "" || fileName != filepath.Base(fileName) || strings.ContainsAny(fileName, `/\`) || strings.HasPrefix(fileName, ".") {
		return Resolved{}, fmt.Errorf("%s: the executable has the file name %q, which is empty, hidden or contains a path separator", ref, fileName)
	}
	return Resolved{
		Reference:      ref,
		Platform:       platform,
		IndexDigest:    indexDesc.Digest.String(),
		ManifestDigest: selected.Digest.String(),
		Manifest:       pluginManifest,
		FileName:       fileName,
		layer:          manifest.Layers[0],
	}, nil
}

func (c *Client) fetchIndex(ctx context.Context, repo *remote.Repository, ref Reference) (ocispec.Descriptor, ocispec.Index, error) {
	desc, reader, err := repo.Manifests().FetchReference(ctx, ref.Tag)
	if err != nil {
		if errors.Is(err, errdef.ErrNotFound) {
			return ocispec.Descriptor{}, ocispec.Index{}, fmt.Errorf("%s: %w", ref, ErrNotFound)
		}
		return ocispec.Descriptor{}, ocispec.Index{}, fmt.Errorf("%s: %w", ref, err)
	}
	defer reader.Close()
	if desc.MediaType != ocispec.MediaTypeImageIndex {
		return ocispec.Descriptor{}, ocispec.Index{}, fmt.Errorf("%s is not an sf plugin: the tag points to the media type %s, not an OCI image index", ref, desc.MediaType)
	}
	data, err := content.ReadAll(reader, desc)
	if err != nil {
		return ocispec.Descriptor{}, ocispec.Index{}, fmt.Errorf("%s: %w", ref, err)
	}
	var index ocispec.Index
	if err := json.Unmarshal(data, &index); err != nil {
		return ocispec.Descriptor{}, ocispec.Index{}, fmt.Errorf("%s: the index is not valid JSON: %w", ref, err)
	}
	return desc, index, nil
}

// Fetch downloads the executable of a resolved plugin into the store and
// returns its path.  The bytes are checked against the digest in the
// manifest before the file is given its final name.  An executable that is
// already in the store is not downloaded again.
func (c *Client) Fetch(ctx context.Context, resolved Resolved, store *Store) (string, error) {
	dir, err := store.digestDir(resolved.ManifestDigest)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, resolved.FileName)
	if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && info.Size() == resolved.layer.Size {
		return path, nil
	}
	repo, err := c.repository(resolved.Reference)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	reader, err := repo.Blobs().Fetch(ctx, resolved.layer)
	if err != nil {
		return "", fmt.Errorf("%s: downloading the executable: %w", resolved.Reference, err)
	}
	defer reader.Close()
	temp, err := os.CreateTemp(dir, "."+resolved.FileName+".*")
	if err != nil {
		return "", err
	}
	defer os.Remove(temp.Name())
	verified := content.NewVerifyReader(reader, resolved.layer)
	if _, err := io.Copy(temp, verified); err != nil {
		temp.Close()
		return "", fmt.Errorf("%s: downloading the executable: %w", resolved.Reference, err)
	}
	if err := verified.Verify(); err != nil {
		temp.Close()
		return "", fmt.Errorf("%s: the downloaded executable does not match the digest in its manifest: %w", resolved.Reference, err)
	}
	if err := temp.Close(); err != nil {
		return "", err
	}
	if err := os.Chmod(temp.Name(), 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(temp.Name(), path); err != nil {
		return "", err
	}
	return path, nil
}

// PublishOptions are the optional parts of a publish.
type PublishOptions struct {
	// Source is the URL of the source repository.  GitHub Container
	// Registry links a package to the repository named here.
	Source string
	// Revision is the source control revision the executables were built
	// from.
	Revision string
	// Overwrite allows the tag to be moved when it already exists.
	Overwrite bool
}

// PublishResult describes a published plugin.
type PublishResult struct {
	Reference   Reference
	IndexDigest string
	// Platforms maps "os/arch" to the digest of its manifest.
	Platforms map[string]string
	// Replaced is the digest the tag pointed to before, when the tag
	// existed.
	Replaced string
}

// Publish uploads one executable for each platform and points the tag of
// ref at an index of them.
func (c *Client) Publish(ctx context.Context, ref Reference, manifest sfplugin.Manifest, executables map[Platform]string, opts PublishOptions) (PublishResult, error) {
	if err := manifest.Validate(); err != nil {
		return PublishResult{}, err
	}
	if len(executables) == 0 {
		return PublishResult{}, fmt.Errorf("no executables to publish")
	}
	repo, err := c.repository(ref)
	if err != nil {
		return PublishResult{}, err
	}
	result := PublishResult{Reference: ref, Platforms: map[string]string{}}
	existing, _, err := c.fetchIndex(ctx, repo, ref)
	switch {
	case err == nil:
		if !opts.Overwrite {
			return PublishResult{}, fmt.Errorf("%s already exists with the digest %s; publish a new tag, or allow the tag to be moved with --overwrite", ref, existing.Digest)
		}
		result.Replaced = existing.Digest.String()
	case errors.Is(err, ErrNotFound):
	default:
		// A tag that points to something other than an index is replaced
		// only on request.
		if !opts.Overwrite {
			return PublishResult{}, err
		}
	}

	configData, err := json.Marshal(manifest)
	if err != nil {
		return PublishResult{}, err
	}
	configDesc := content.NewDescriptorFromBytes(ConfigMediaType, configData)
	if err := pushIfMissing(ctx, repo, configDesc, func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(configData)), nil
	}); err != nil {
		return PublishResult{}, fmt.Errorf("%s: uploading the plugin manifest: %w", ref, err)
	}

	annotations := map[string]string{ocispec.AnnotationVersion: ref.Tag}
	if opts.Source != "" {
		annotations[ocispec.AnnotationSource] = opts.Source
	}
	if opts.Revision != "" {
		annotations[ocispec.AnnotationRevision] = opts.Revision
	}

	platforms := make([]Platform, 0, len(executables))
	for platform := range executables {
		platforms = append(platforms, platform)
	}
	sort.Slice(platforms, func(i, j int) bool { return platforms[i].String() < platforms[j].String() })

	index := ocispec.Index{
		Versioned:    specs.Versioned{SchemaVersion: 2},
		MediaType:    ocispec.MediaTypeImageIndex,
		ArtifactType: ArtifactType,
		Annotations:  annotations,
	}
	for _, platform := range platforms {
		path := executables[platform]
		layer, err := fileDescriptor(path)
		if err != nil {
			return PublishResult{}, err
		}
		layer.Annotations = map[string]string{ocispec.AnnotationTitle: ExecutableName(manifest.Name, platform)}
		if err := pushIfMissing(ctx, repo, layer, func() (io.ReadCloser, error) { return os.Open(path) }); err != nil {
			return PublishResult{}, fmt.Errorf("%s: uploading %s: %w", ref, path, err)
		}
		manifestAnnotations := map[string]string{}
		for key, value := range annotations {
			manifestAnnotations[key] = value
		}
		manifestDesc, err := oras.PackManifest(ctx, repo, oras.PackManifestVersion1_1, ArtifactType, oras.PackManifestOptions{
			Layers:              []ocispec.Descriptor{layer},
			ConfigDescriptor:    &configDesc,
			ManifestAnnotations: manifestAnnotations,
		})
		if err != nil {
			return PublishResult{}, fmt.Errorf("%s: uploading the manifest for %s: %w", ref, platform, err)
		}
		manifestDesc.Platform = &ocispec.Platform{OS: platform.OS, Architecture: platform.Arch}
		manifestDesc.Annotations = nil
		index.Manifests = append(index.Manifests, manifestDesc)
		result.Platforms[platform.String()] = manifestDesc.Digest.String()
	}

	indexData, err := json.Marshal(index)
	if err != nil {
		return PublishResult{}, err
	}
	indexDesc := content.NewDescriptorFromBytes(ocispec.MediaTypeImageIndex, indexData)
	if err := repo.Manifests().PushReference(ctx, indexDesc, bytes.NewReader(indexData), ref.Tag); err != nil {
		return PublishResult{}, fmt.Errorf("%s: uploading the index: %w", ref, err)
	}
	result.IndexDigest = indexDesc.Digest.String()
	return result, nil
}

func pushIfMissing(ctx context.Context, repo *remote.Repository, desc ocispec.Descriptor, open func() (io.ReadCloser, error)) error {
	exists, err := repo.Blobs().Exists(ctx, desc)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	reader, err := open()
	if err != nil {
		return err
	}
	defer reader.Close()
	return repo.Blobs().Push(ctx, desc, reader)
}

// fileDescriptor returns the descriptor of an executable file: its size
// and SHA-256 digest.
func fileDescriptor(path string) (ocispec.Descriptor, error) {
	file, err := os.Open(path)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	if !info.Mode().IsRegular() {
		return ocispec.Descriptor{}, fmt.Errorf("%s is not a regular file", path)
	}
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	return ocispec.Descriptor{
		MediaType: ExecutableMediaType,
		Digest:    digest.NewDigestFromEncoded(digest.SHA256, hex.EncodeToString(hash.Sum(nil))),
		Size:      size,
	}, nil
}
