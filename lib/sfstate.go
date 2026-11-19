package lib

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

// This file reads and writes the state files of the Salesforce sf CLI:
// config (~/.sf/config.json and <project>/.sf/config.json), aliases
// (~/.sfdx/alias.json) and org authorization files (~/.sfdx/<username>.json).

const (
	SFConfigTargetOrg    = "target-org"
	SFConfigTargetDevHub = "target-dev-hub"
	SFConfigApiVersion   = "org-api-version"

	SFConfigLocationGlobal = "Global"
	SFConfigLocationLocal  = "Local"
	SFConfigLocationEnv    = "Environment"
)

// SFConfigKeys lists the config names sf accepts.
var SFConfigKeys = []string{
	"disable-telemetry",
	"org-api-version",
	"org-capitalize-record-types",
	"org-custom-metadata-templates",
	"org-instance-url",
	"org-isv-debugger-sid",
	"org-isv-debugger-url",
	"org-max-query-limit",
	"org-metadata-rest-deploy",
	"target-dev-hub",
	"target-org",
}

// sfConfigEnvVars maps a config name to the environment variables that
// override it, in the order sf checks them.
var sfConfigEnvVars = map[string][]string{
	"target-org":          {"SF_TARGET_ORG", "SFDX_DEFAULTUSERNAME"},
	"target-dev-hub":      {"SF_TARGET_DEV_HUB", "SFDX_DEFAULTDEVHUBUSERNAME"},
	"org-api-version":     {"SF_ORG_API_VERSION", "SFDX_API_VERSION"},
	"org-instance-url":    {"SF_ORG_INSTANCE_URL", "SFDX_INSTANCE_URL"},
	"org-max-query-limit": {"SF_ORG_MAX_QUERY_LIMIT", "SFDX_MAX_QUERY_LIMIT"},
	"disable-telemetry":   {"SF_DISABLE_TELEMETRY", "SFDX_DISABLE_TELEMETRY"},
}

// sfdxConfigKeys maps sf config names to the names used in the sfdx config
// files (.sfdx/sfdx-config.json).
var sfdxConfigKeys = map[string]string{
	"target-org":        "defaultusername",
	"target-dev-hub":    "defaultdevhubusername",
	"org-api-version":   "apiVersion",
	"org-instance-url":  "instanceUrl",
	"disable-telemetry": "disableTelemetry",
}

// SFConfigValue is one resolved config value and where it came from.
type SFConfigValue struct {
	Key      string
	Value    string
	Location string
	Path     string
}

// IsSFConfigKey reports whether key is a config name sf accepts.
func IsSFConfigKey(key string) bool {
	for _, known := range SFConfigKeys {
		if known == key {
			return true
		}
	}
	return false
}

// SFProjectRoot returns the nearest directory at or above dir that contains
// sfdx-project.json.
func SFProjectRoot(dir string) (string, bool) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "sfdx-project.json")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// SFGlobalConfigPath returns ~/.sf/config.json.
func SFGlobalConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".sf", "config.json"), nil
}

// SFLocalConfigPath returns <project>/.sf/config.json for the project that
// contains the working directory.
func SFLocalConfigPath() (string, bool) {
	wd, err := os.Getwd()
	if err != nil {
		return "", false
	}
	root, ok := SFProjectRoot(wd)
	if !ok {
		return "", false
	}
	return filepath.Join(root, ".sf", "config.json"), true
}

func readSFConfigFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	raw := map[string]interface{}{}
	if len(strings.TrimSpace(string(data))) > 0 {
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
	}
	values := make(map[string]string, len(raw))
	for key, value := range raw {
		switch v := value.(type) {
		case string:
			values[key] = v
		case nil:
		default:
			encoded, _ := json.Marshal(v)
			values[key] = string(encoded)
		}
	}
	return values, nil
}

func writeSFConfigFile(path string, values map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// sfdxConfigFallback reads the sfdx config file next to an sf config file
// (<dir>/../.sfdx/sfdx-config.json) and returns its values under sf names.
func sfdxConfigFallback(sfConfigPath string) map[string]string {
	base := filepath.Dir(filepath.Dir(sfConfigPath))
	legacy, err := readSFConfigFile(filepath.Join(base, ".sfdx", "sfdx-config.json"))
	if err != nil {
		return nil
	}
	values := map[string]string{}
	for key, legacyKey := range sfdxConfigKeys {
		if value, ok := legacy[legacyKey]; ok {
			values[key] = value
		}
	}
	return values
}

// ListSFConfig returns every config value that is set, sorted by name.  A
// value from the environment replaces a local value, and a local value
// replaces a global value.
func ListSFConfig() ([]SFConfigValue, error) {
	resolved := map[string]SFConfigValue{}
	globalPath, err := SFGlobalConfigPath()
	if err != nil {
		return nil, err
	}
	sources := []struct {
		path     string
		location string
	}{{globalPath, SFConfigLocationGlobal}}
	if localPath, ok := SFLocalConfigPath(); ok {
		sources = append(sources, struct {
			path     string
			location string
		}{localPath, SFConfigLocationLocal})
	}
	for _, source := range sources {
		for key, value := range sfdxConfigFallback(source.path) {
			resolved[key] = SFConfigValue{Key: key, Value: value, Location: source.location, Path: source.path}
		}
		values, err := readSFConfigFile(source.path)
		if err != nil {
			return nil, err
		}
		for key, value := range values {
			resolved[key] = SFConfigValue{Key: key, Value: value, Location: source.location, Path: source.path}
		}
	}
	for key, names := range sfConfigEnvVars {
		for _, name := range names {
			if value := os.Getenv(name); value != "" {
				resolved[key] = SFConfigValue{Key: key, Value: value, Location: SFConfigLocationEnv, Path: "$" + name}
				break
			}
		}
	}
	keys := make([]string, 0, len(resolved))
	for key := range resolved {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]SFConfigValue, 0, len(keys))
	for _, key := range keys {
		result = append(result, resolved[key])
	}
	return result, nil
}

// GetSFConfig returns the resolved value of one config name.
func GetSFConfig(key string) (SFConfigValue, bool, error) {
	values, err := ListSFConfig()
	if err != nil {
		return SFConfigValue{}, false, err
	}
	for _, value := range values {
		if value.Key == key {
			return value, true, nil
		}
	}
	return SFConfigValue{}, false, nil
}

// SFConfigFileValues returns the values stored in the global or local
// config file, without environment overrides, and the path of the file.
func SFConfigFileValues(global bool) (map[string]string, string, error) {
	path, err := sfConfigPathFor(global)
	if err != nil {
		return nil, "", err
	}
	values, err := readSFConfigFile(path)
	return values, path, err
}

func sfConfigPathFor(global bool) (string, error) {
	if global {
		return SFGlobalConfigPath()
	}
	path, ok := SFLocalConfigPath()
	if !ok {
		return "", ErrNotInSFProject
	}
	return path, nil
}

// ErrNotInSFProject is returned when a local config file is requested
// outside a directory tree that contains sfdx-project.json.
var ErrNotInSFProject = errors.New("this directory does not contain a valid Salesforce DX project")

// SetSFConfig writes a config value to the global or local config file and
// returns the path of the file.
func SetSFConfig(key, value string, global bool) (string, error) {
	path, err := sfConfigPathFor(global)
	if err != nil {
		return "", err
	}
	values, err := readSFConfigFile(path)
	if err != nil {
		return "", err
	}
	values[key] = value
	return path, writeSFConfigFile(path, values)
}

// UnsetSFConfig removes a config value from the global or local config file
// and returns the path of the file.
func UnsetSFConfig(key string, global bool) (string, error) {
	path, err := sfConfigPathFor(global)
	if err != nil {
		return "", err
	}
	values, err := readSFConfigFile(path)
	if err != nil {
		return "", err
	}
	delete(values, key)
	return path, writeSFConfigFile(path, values)
}

// sfdxWriteDir returns the directory sf writes alias and auth files to.
func sfdxWriteDir() (string, error) {
	dirs := sfdxStateDirs()
	if len(dirs) == 0 {
		return "", fmt.Errorf("unable to determine SFDX state directories")
	}
	return dirs[0], nil
}

// SFDXAlias is one alias and the username it refers to.
type SFDXAlias struct {
	Alias string
	Value string
}

// ListSFDXAliasesOrdered returns the aliases in the order they appear in
// alias.json, which is the order sf lists them in.
func ListSFDXAliasesOrdered() ([]SFDXAlias, error) {
	dir, err := sfdxWriteDir()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, sfdxAliasFilename))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return parseOrderedAliases(data)
}

func parseOrderedAliases(data []byte) ([]SFDXAlias, error) {
	var file struct {
		Orgs json.RawMessage `json:"orgs"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	if len(file.Orgs) == 0 {
		return nil, nil
	}
	dec := json.NewDecoder(strings.NewReader(string(file.Orgs)))
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	var aliases []SFDXAlias
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return nil, err
		}
		var value interface{}
		if err := dec.Decode(&value); err != nil {
			return nil, err
		}
		if s, ok := value.(string); ok {
			aliases = append(aliases, SFDXAlias{Alias: key.(string), Value: s})
		}
	}
	return aliases, nil
}

func writeSFDXAliases(aliases []SFDXAlias) error {
	dir, err := sfdxWriteDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString(`{"orgs":{`)
	for i, alias := range aliases {
		if i > 0 {
			b.WriteString(",")
		}
		key, _ := json.Marshal(alias.Alias)
		value, _ := json.Marshal(alias.Value)
		b.Write(key)
		b.WriteString(":")
		b.Write(value)
	}
	b.WriteString("}}")
	return os.WriteFile(filepath.Join(dir, sfdxAliasFilename), []byte(b.String()), 0o600)
}

// SetSFDXAlias points alias at username, replacing an existing alias of the
// same name.
func SetSFDXAlias(alias, username string) error {
	aliases, err := ListSFDXAliasesOrdered()
	if err != nil {
		return err
	}
	for i := range aliases {
		if aliases[i].Alias == alias {
			aliases[i].Value = username
			return writeSFDXAliases(aliases)
		}
	}
	return writeSFDXAliases(append(aliases, SFDXAlias{Alias: alias, Value: username}))
}

// UnsetSFDXAlias removes alias and returns the username it referred to.
func UnsetSFDXAlias(alias string) (string, bool, error) {
	aliases, err := ListSFDXAliasesOrdered()
	if err != nil {
		return "", false, err
	}
	for i := range aliases {
		if aliases[i].Alias == alias {
			value := aliases[i].Value
			return value, true, writeSFDXAliases(append(aliases[:i], aliases[i+1:]...))
		}
	}
	return "", false, nil
}

// ResolveSFDXUsername returns the username for an alias, or the argument
// itself when it is not an alias.
func ResolveSFDXUsername(nameOrAlias string) string {
	if username, ok := ListSFDXAliases()[nameOrAlias]; ok {
		return username
	}
	return nameOrAlias
}

// sfdxEncryptedField matches the auth file fields sf encrypts.
var sfdxEncryptedField = regexp.MustCompile(`(?i)token|password|secret`)

// SFDXAuthFields is the content of an org authorization file with the
// encrypted fields decrypted.  Field names are those sf writes, such as
// accessToken, instanceUrl, orgId and isDevHub.
type SFDXAuthFields map[string]interface{}

// String returns a string field, or "" when it is absent or not a string.
func (f SFDXAuthFields) String(key string) string {
	value, _ := f[key].(string)
	return value
}

// Bool returns a boolean field, or false when it is absent.
func (f SFDXAuthFields) Bool(key string) bool {
	value, _ := f[key].(bool)
	return value
}

func sfdxAuthPath(username string) (string, error) {
	dir, err := sfdxWriteDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, username+".json"), nil
}

// ReadSFDXAuthFields reads the authorization file for username and decrypts
// the encrypted fields.
func ReadSFDXAuthFields(username string) (SFDXAuthFields, error) {
	fields, _, err := ReadSFDXAuthFieldsOrdered(username)
	return fields, err
}

// SFDXAuthModTime returns the modification time of the authorization file
// for username.  sf reports it as the time the org was last used.
func SFDXAuthModTime(username string) (time.Time, error) {
	for _, dir := range sfdxStateDirs() {
		for _, candidate := range []string{filepath.Join(dir, username+".json"), filepath.Join(dir, "orgs", username+".json")} {
			if info, err := os.Stat(candidate); err == nil {
				return info.ModTime(), nil
			}
		}
	}
	return time.Time{}, fmt.Errorf("could not locate SFDX auth file for %s", username)
}

// ReadSFDXAuthFieldsOrdered is ReadSFDXAuthFields and also returns the field
// names in the order they appear in the file.
func ReadSFDXAuthFieldsOrdered(username string) (SFDXAuthFields, []string, error) {
	stateDirs := sfdxStateDirs()
	var (
		data     []byte
		stateDir string
		found    bool
	)
	for _, dir := range stateDirs {
		for _, candidate := range []string{filepath.Join(dir, username+".json"), filepath.Join(dir, "orgs", username+".json")} {
			content, err := os.ReadFile(candidate)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					continue
				}
				return nil, nil, err
			}
			data, stateDir, found = content, dir, true
			break
		}
		if found {
			break
		}
	}
	if !found {
		return nil, nil, fmt.Errorf("could not locate SFDX auth file for %s", username)
	}
	fields := SFDXAuthFields{}
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, nil, err
	}
	var keys []sfdxKey
	keyIndex := -1
	for name, value := range fields {
		text, ok := value.(string)
		if !ok || !sfdxEncryptedField.MatchString(name) || !isEncryptedFormat(text) {
			continue
		}
		if keys == nil {
			loaded, err := loadSFDXKeys(stateDir)
			if err != nil {
				return nil, nil, err
			}
			keys = loaded
		}
		decrypted, index, err := decryptWithCandidates(text, keyIndex, keys)
		if err != nil {
			return nil, nil, err
		}
		keyIndex = index
		fields[name] = decrypted
	}
	order := jsonObjectKeys(data)
	if fields.String("username") == "" {
		fields["username"] = username
		order = append(order, "username")
	}
	return fields, order, nil
}

// jsonObjectKeys returns the keys of a JSON object in document order.
func jsonObjectKeys(data []byte) []string {
	dec := json.NewDecoder(strings.NewReader(string(data)))
	if _, err := dec.Token(); err != nil {
		return nil
	}
	var keys []string
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return keys
		}
		name, ok := key.(string)
		if !ok {
			return keys
		}
		keys = append(keys, name)
		var skipped json.RawMessage
		if err := dec.Decode(&skipped); err != nil {
			return keys
		}
	}
	return keys
}

// sfdxAuthFieldOrder is the order sf writes the common fields in after a web
// login.  It is used for a new file when the caller gives no order.
var sfdxAuthFieldOrder = []string{
	"accessToken", "instanceUrl", "orgId", "username", "loginUrl", "refreshToken",
	"clientId", "clientSecret", "privateKey", "isDevHub", "instanceApiVersion",
	"instanceApiVersionLastRetrieved", "name", "instanceName", "namespacePrefix",
	"isSandbox", "isScratch", "trailExpirationDate", "orgEdition", "tracksSource",
}

// WriteSFDXAuthFields writes the authorization file for the username in
// fields, encrypting the fields sf encrypts.  It replaces an existing file.
// Fields are written in the given order; fields not named in order follow in
// alphabetical order.
func WriteSFDXAuthFields(fields SFDXAuthFields, order ...string) error {
	username := fields.String("username")
	if username == "" {
		return fmt.Errorf("authorization has no username")
	}
	path, err := sfdxAuthPath(username)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	var key *sfdxKey
	names := make([]string, 0, len(fields))
	seen := map[string]bool{}
	if len(order) == 0 {
		order = sfdxAuthFieldOrder
	}
	for _, name := range order {
		if _, ok := fields[name]; ok && !seen[name] {
			names = append(names, name)
			seen[name] = true
		}
	}
	var others []string
	for name := range fields {
		if !seen[name] {
			others = append(others, name)
		}
	}
	sort.Strings(others)
	names = append(names, others...)

	var b strings.Builder
	b.WriteString("{\n")
	for i, name := range names {
		value := fields[name]
		if text, ok := value.(string); ok && text != "" && sfdxEncryptedField.MatchString(name) {
			if key == nil {
				loaded, err := sfdxEncryptionKey(dir)
				if err != nil {
					return err
				}
				key = &loaded
			}
			encrypted, err := encryptSFDXField(text, key.value, key.version)
			if err != nil {
				return err
			}
			value = encrypted
		}
		encodedName, _ := json.Marshal(name)
		encodedValue, err := marshalNoEscape(value)
		if err != nil {
			return err
		}
		b.WriteString("  ")
		b.Write(encodedName)
		b.WriteString(": ")
		b.Write(encodedValue)
		if i < len(names)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("}")
	return os.WriteFile(path, []byte(b.String()), 0o600)
}

func marshalNoEscape(value interface{}) ([]byte, error) {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); err != nil {
		return nil, err
	}
	return []byte(strings.TrimRight(b.String(), "\n")), nil
}

// UpdateSFDXAuthFields changes fields in the authorization file for
// username and keeps the other fields.
func UpdateSFDXAuthFields(username string, updates SFDXAuthFields) error {
	fields, order, err := ReadSFDXAuthFieldsOrdered(username)
	if err != nil {
		return err
	}
	for name, value := range updates {
		fields[name] = value
	}
	return WriteSFDXAuthFields(fields, order...)
}

// DeleteSFDXAuth removes the authorization file for username from every sf
// state directory.
func DeleteSFDXAuth(username string) error {
	removed := false
	for _, dir := range sfdxStateDirs() {
		for _, candidate := range []string{filepath.Join(dir, username+".json"), filepath.Join(dir, "orgs", username+".json")} {
			err := os.Remove(candidate)
			if err == nil {
				removed = true
			} else if !errors.Is(err, fs.ErrNotExist) {
				return err
			}
		}
	}
	if !removed {
		return fmt.Errorf("could not locate SFDX auth file for %s", username)
	}
	return nil
}

// ListSFDXUsernames returns the usernames that have an authorization file.
func ListSFDXUsernames() []string {
	return listSFDXUsernames()
}

func useGenericKeychain() bool {
	for _, name := range []string{"SF_USE_GENERIC_UNIX_KEYCHAIN", "SFDX_USE_GENERIC_UNIX_KEYCHAIN", "USE_GENERIC_UNIX_KEYCHAIN"} {
		if strings.EqualFold(os.Getenv(name), "true") {
			return true
		}
	}
	return false
}

// sfdxEncryptionKey returns the key sf uses to encrypt auth file fields:
// the key in the operating system keychain, or the key in key.json when the
// keychain has none or SF_USE_GENERIC_UNIX_KEYCHAIN is true.  When no key
// exists it creates one, in the keychain if that works and in key.json
// otherwise.
func sfdxEncryptionKey(stateDir string) (sfdxKey, error) {
	generic := useGenericKeychain()
	if !generic {
		if value, version, err := loadSFDXKeyFromKeychain(); err == nil {
			return sfdxKey{value: value, version: version}, nil
		} else if err != errSFDXKeyNotFound {
			return sfdxKey{}, err
		}
	}
	if value, version, err := loadSFDXKeyFromFile(stateDir); err == nil {
		return sfdxKey{value: value, version: version}, nil
	} else if err != errSFDXKeyNotFound {
		return sfdxKey{}, err
	}

	// sf creates a version 1 key (32 hexadecimal characters) unless
	// SF_CRYPTO_V2 is true.
	size := 16
	if strings.EqualFold(os.Getenv("SF_CRYPTO_V2"), "true") {
		size = 32
	}
	random := make([]byte, size)
	if _, err := rand.Read(random); err != nil {
		return sfdxKey{}, err
	}
	encoded := hex.EncodeToString(random)
	value, version, err := parseSFDXKey(encoded)
	if err != nil {
		return sfdxKey{}, err
	}
	if generic || storeSFDXKeyInKeychain(encoded) != nil {
		keyFile := map[string]string{"service": sfdxKeyService, "account": sfdxKeyAccount, "key": encoded}
		data, _ := json.Marshal(keyFile)
		if err := os.MkdirAll(stateDir, 0o700); err != nil {
			return sfdxKey{}, err
		}
		if err := os.WriteFile(filepath.Join(stateDir, sfdxKeyFilename), data, 0o600); err != nil {
			return sfdxKey{}, err
		}
	}
	return sfdxKey{value: value, version: version}, nil
}

func storeSFDXKeyInKeychain(key string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("security", "add-generic-password", "-a", sfdxKeyAccount, "-s", sfdxKeyService, "-w", key).Run()
	case "linux":
		cmdName := os.Getenv(secretToolEnvVar)
		if strings.TrimSpace(cmdName) == "" {
			cmdName = "secret-tool"
		}
		cmd := exec.Command(cmdName, "store", "--label="+sfdxKeyService, "user", sfdxKeyAccount, "domain", sfdxKeyService)
		cmd.Env = secretToolEnvironment()
		cmd.Stdin = strings.NewReader(key)
		return cmd.Run()
	default:
		return fmt.Errorf("no keychain support on %s", runtime.GOOS)
	}
}

// encryptSFDXField encrypts value in the format decryptSFDXField reads:
// the initialization vector, the hexadecimal ciphertext, a colon and the
// hexadecimal authentication tag.
func encryptSFDXField(value string, key []byte, version sfdxCryptoVersion) (string, error) {
	var (
		iv     []byte
		prefix string
	)
	switch version {
	case cryptoV1:
		random := make([]byte, 6)
		if _, err := rand.Read(random); err != nil {
			return "", err
		}
		prefix = hex.EncodeToString(random)
		iv = []byte(prefix)
	case cryptoV2:
		iv = make([]byte, 12)
		if _, err := rand.Read(iv); err != nil {
			return "", err
		}
		prefix = hex.EncodeToString(iv)
	default:
		return "", fmt.Errorf("unsupported SFDX crypto version")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCMWithNonceSize(block, len(iv))
	if err != nil {
		return "", err
	}
	sealed := gcm.Seal(nil, iv, []byte(value), nil)
	tagStart := len(sealed) - gcm.Overhead()
	return prefix + hex.EncodeToString(sealed[:tagStart]) + ":" + hex.EncodeToString(sealed[tagStart:]), nil
}
