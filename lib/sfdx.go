package lib

import (
	"crypto/aes"
	"crypto/cipher"
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
)

type SFDXAuth struct {
	AccessToken        string
	Alias              string
	ClientId           string
	CreatedBy          string
	DevHubId           string
	Edition            string
	Id                 string
	InstanceUrl        string
	OrgName            string
	Password           string
	Status             string
	Username           string
	LoginUrl           string
	RefreshToken       string
	UserId             string
	InstanceApiVersion string
}

const (
	sfdxAliasFilename = "alias.json"
	sfdxKeyFilename   = "key.json"
	sfdxKeyService    = "sfdx"
	sfdxKeyAccount    = "local"
	secretToolEnvVar  = "SFDX_SECRET_TOOL_PATH"
)

type sfdxAuthFile struct {
	AccessToken        string `json:"accessToken"`
	InstanceUrl        string `json:"instanceUrl"`
	LoginUrl           string `json:"loginUrl"`
	OrgId              string `json:"orgId"`
	Id                 string `json:"id"`
	Username           string `json:"username"`
	UserId             string `json:"userId"`
	ClientId           string `json:"clientId"`
	ClientSecret       string `json:"clientSecret"`
	RefreshToken       string `json:"refreshToken"`
	Password           string `json:"password"`
	InstanceApiVersion string `json:"instanceApiVersion"`
	Alias              string `json:"alias"`
}

type sfdxCryptoVersion int

const (
	cryptoUnknown sfdxCryptoVersion = iota
	cryptoV1
	cryptoV2
)

var errSFDXKeyNotFound = errors.New("sfdx crypto key not found")

type sfdxKey struct {
	value   []byte
	version sfdxCryptoVersion
}

func sfdxStateDirs() []string {
	if override := os.Getenv("FORCE_SFDX_STATE_DIRS"); override != "" {
		var dirs []string
		for _, entry := range filepath.SplitList(override) {
			entry = strings.TrimSpace(entry)
			if entry != "" {
				dirs = append(dirs, entry)
			}
		}
		if len(dirs) > 0 {
			return dirs
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{
		filepath.Join(home, ".sfdx"),
		filepath.Join(home, ".sf"),
	}
}

func loadSFDXAliases(stateDirs []string) map[string]string {
	aliases := make(map[string]string)
	for _, dir := range stateDirs {
		if dir == "" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, sfdxAliasFilename))
		if err != nil {
			continue
		}
		var aliasFile struct {
			Orgs map[string]string `json:"orgs"`
		}
		if err := json.Unmarshal(data, &aliasFile); err != nil {
			continue
		}
		for alias, username := range aliasFile.Orgs {
			alias = strings.TrimSpace(alias)
			username = strings.TrimSpace(username)
			if alias == "" || username == "" {
				continue
			}
			if _, exists := aliases[alias]; !exists {
				aliases[alias] = username
			}
		}
	}
	return aliases
}

func ListSFDXAliases() map[string]string {
	aliases := loadSFDXAliases(sfdxStateDirs())
	result := make(map[string]string, len(aliases))
	for k, v := range aliases {
		result[k] = v
	}
	return result
}

func resolveSFDXUsername(user string, aliases map[string]string) (string, string) {
	trimmed := strings.TrimSpace(user)
	if trimmed == "" {
		return "", ""
	}
	if actual, ok := aliases[trimmed]; ok {
		return actual, trimmed
	}
	alias := ""
	for candidateAlias, candidateUsername := range aliases {
		if candidateUsername == trimmed {
			alias = candidateAlias
			break
		}
	}
	if alias == "" {
		for candidateAlias, candidateUsername := range aliases {
			if strings.EqualFold(candidateUsername, trimmed) {
				alias = candidateAlias
				break
			}
		}
	}
	return trimmed, alias
}

func findAliasForUsername(aliases map[string]string, username string) string {
	username = strings.TrimSpace(username)
	if username == "" {
		return ""
	}
	for alias, candidate := range aliases {
		if candidate == username {
			return alias
		}
	}
	for alias, candidate := range aliases {
		if strings.EqualFold(candidate, username) {
			return alias
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func loadSFDXAuthFile(username string, stateDirs []string) (sfdxAuthFile, string, error) {
	trimmed := strings.TrimSpace(username)
	if trimmed == "" {
		return sfdxAuthFile{}, "", fmt.Errorf("no SFDX username provided")
	}

	for _, dir := range stateDirs {
		if dir == "" {
			continue
		}
		candidates := []string{
			filepath.Join(dir, trimmed+".json"),
			filepath.Join(dir, "orgs", trimmed+".json"),
		}
		for _, candidate := range candidates {
			data, err := os.ReadFile(candidate)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					continue
				}
				return sfdxAuthFile{}, "", err
			}
			var auth sfdxAuthFile
			if err := json.Unmarshal(data, &auth); err != nil {
				return sfdxAuthFile{}, "", err
			}
			if auth.Username == "" {
				auth.Username = trimmed
			}
			if auth.OrgId == "" {
				auth.OrgId = auth.Id
			}
			return auth, dir, nil
		}
	}
	return sfdxAuthFile{}, "", fmt.Errorf("could not locate SFDX auth file for %s", username)
}

func loadSFDXKeys(stateDir string) ([]sfdxKey, error) {
	seen := make(map[string]struct{})
	var keys []sfdxKey

	if value, version, err := loadSFDXKeyFromFile(stateDir); err == nil {
		keys = appendSFDXKey(keys, value, version, seen)
	} else if err != errSFDXKeyNotFound {
		return nil, err
	}

	if value, version, err := loadSFDXKeyFromKeychain(); err == nil {
		keys = appendSFDXKey(keys, value, version, seen)
	} else if err != errSFDXKeyNotFound {
		return nil, err
	}

	if len(keys) == 0 {
		return nil, errSFDXKeyNotFound
	}

	return keys, nil
}

func loadSFDXKeyFromFile(stateDir string) ([]byte, sfdxCryptoVersion, error) {
	candidateDirs := make([]string, 0, 4)
	seen := map[string]struct{}{}
	if trimmed := strings.TrimSpace(stateDir); trimmed != "" {
		candidateDirs = append(candidateDirs, trimmed)
		seen[filepath.Clean(trimmed)] = struct{}{}
	}
	for _, dir := range sfdxStateDirs() {
		if dir == "" {
			continue
		}
		clean := filepath.Clean(dir)
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		candidateDirs = append(candidateDirs, dir)
	}

	for _, dir := range candidateDirs {
		data, err := os.ReadFile(filepath.Join(dir, sfdxKeyFilename))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, cryptoUnknown, err
		}
		var keyFile struct {
			Key string `json:"key"`
		}
		if err := json.Unmarshal(data, &keyFile); err != nil {
			return nil, cryptoUnknown, err
		}
		if key, version, err := parseSFDXKey(keyFile.Key); err == nil {
			return key, version, nil
		}
	}
	return nil, cryptoUnknown, errSFDXKeyNotFound
}

func appendSFDXKey(keys []sfdxKey, value []byte, version sfdxCryptoVersion, seen map[string]struct{}) []sfdxKey {
	if value == nil {
		return keys
	}
	keyStr := string(value)
	if _, exists := seen[keyStr]; exists {
		return keys
	}
	seen[keyStr] = struct{}{}
	copied := make([]byte, len(value))
	copy(copied, value)
	return append(keys, sfdxKey{value: copied, version: version})
}

func secretToolEnvironment() []string {
	keys := []string{
		"PATH",
		"HOME",
		"DBUS_SESSION_BUS_ADDRESS",
		"DISPLAY",
		"XDG_RUNTIME_DIR",
		"XDG_SESSION_TYPE",
	}
	env := make([]string, 0, len(keys))
	for _, key := range keys {
		val := strings.TrimSpace(os.Getenv(key))
		if val != "" {
			env = append(env, key+"="+val)
		}
	}
	if len(env) == 0 {
		return os.Environ()
	}
	return env
}

func decryptWithCandidates(value string, preferred int, keys []sfdxKey) (string, int, error) {
	if value == "" || !isEncryptedFormat(value) {
		return value, preferred, nil
	}
	if len(keys) == 0 {
		return "", preferred, fmt.Errorf("no SFDX encryption key available to decrypt value")
	}
	order := make([]int, 0, len(keys))
	if preferred >= 0 && preferred < len(keys) {
		order = append(order, preferred)
	}
	for i := range keys {
		if i == preferred {
			continue
		}
		order = append(order, i)
	}
	var lastErr error
	for _, idx := range order {
		decrypted, err := decryptSFDXField(value, keys[idx].value, keys[idx].version)
		if err == nil {
			return decrypted, idx, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return "", preferred, lastErr
	}
	return "", preferred, fmt.Errorf("unable to decrypt SFDX value")
}

func loadSFDXKeyFromKeychain() ([]byte, sfdxCryptoVersion, error) {
	switch runtime.GOOS {
	case "darwin":
		return loadSFDXKeyFromSecurity()
	case "linux":
		return loadSFDXKeyFromSecretTool()
	default:
		return nil, cryptoUnknown, errSFDXKeyNotFound
	}
}

func loadSFDXKeyFromSecurity() ([]byte, sfdxCryptoVersion, error) {
	cmd := exec.Command("security", "find-generic-password", "-a", sfdxKeyAccount, "-s", sfdxKeyService, "-w")
	output, err := cmd.CombinedOutput()
	if err != nil {
		var execErr *exec.Error
		if errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound) {
			return nil, cryptoUnknown, errSFDXKeyNotFound
		}
		message := strings.TrimSpace(string(output))
		if message != "" {
			if strings.Contains(strings.ToLower(message), "could not be found") {
				return nil, cryptoUnknown, errSFDXKeyNotFound
			}
			return nil, cryptoUnknown, fmt.Errorf("%s: %w", message, err)
		}
		return nil, cryptoUnknown, err
	}
	return parseSFDXKey(string(output))
}

func loadSFDXKeyFromSecretTool() ([]byte, sfdxCryptoVersion, error) {
	cmdName := os.Getenv(secretToolEnvVar)
	if strings.TrimSpace(cmdName) == "" {
		cmdName = "secret-tool"
	}
	args := []string{"lookup", "user", sfdxKeyAccount, "domain", sfdxKeyService}

	var lastMsg string
	for attempts := 0; attempts < 3; attempts++ {
		cmd := exec.Command(cmdName, args...)
		cmd.Env = secretToolEnvironment()
		output, err := cmd.CombinedOutput()
		if err != nil {
			var execErr *exec.Error
			if errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound) {
				return nil, cryptoUnknown, errSFDXKeyNotFound
			}
			exitErr, ok := err.(*exec.ExitError)
			message := strings.TrimSpace(string(output))
			if !ok {
				if message != "" {
					return nil, cryptoUnknown, fmt.Errorf("secret-tool lookup failed: %s: %w", message, err)
				}
				return nil, cryptoUnknown, fmt.Errorf("secret-tool lookup failed: %w", err)
			}
			if message == "" {
				message = exitErr.Error()
			}
			lastMsg = message
			if exitErr.ExitCode() == 1 {
				if strings.Contains(message, "invalid or unencryptable secret") {
					continue
				}
				return nil, cryptoUnknown, errSFDXKeyNotFound
			}
			return nil, cryptoUnknown, fmt.Errorf("secret-tool lookup failed: %s: %w", message, err)
		}
		return parseSFDXKey(string(output))
	}
	if lastMsg != "" {
		return nil, cryptoUnknown, fmt.Errorf("secret-tool lookup failed: %s", lastMsg)
	}
	return nil, cryptoUnknown, fmt.Errorf("secret-tool lookup failed after retries")
}

func parseSFDXKey(raw string) ([]byte, sfdxCryptoVersion, error) {
	keyStr := strings.TrimSpace(raw)
	if keyStr == "" {
		return nil, cryptoUnknown, errSFDXKeyNotFound
	}

	switch len(keyStr) {
	case 32:
		return []byte(keyStr), cryptoV1, nil
	case 64:
		decoded, err := hex.DecodeString(keyStr)
		if err != nil {
			return nil, cryptoUnknown, err
		}
		return decoded, cryptoV2, nil
	default:
		if len(keyStr)%2 == 0 {
			if decoded, err := hex.DecodeString(keyStr); err == nil {
				switch len(decoded) {
				case 16:
					return []byte(keyStr), cryptoV1, nil
				case 32:
					return decoded, cryptoV2, nil
				}
			}
		}
		return nil, cryptoUnknown, fmt.Errorf("unsupported SFDX key length %d", len(keyStr))
	}
}

func isEncryptedFormat(value string) bool {
	tokens := strings.Split(value, ":")
	if len(tokens) != 2 {
		return false
	}
	if len(tokens[1]) != 32 {
		return false
	}
	if _, err := hex.DecodeString(tokens[1]); err != nil {
		return false
	}
	return true
}

func decryptSFDXField(value string, key []byte, version sfdxCryptoVersion) (string, error) {
	if value == "" {
		return "", nil
	}
	if !isEncryptedFormat(value) {
		return value, nil
	}
	if len(key) == 0 || version == cryptoUnknown {
		return "", fmt.Errorf("encrypted value but crypto key unavailable")
	}

	tokens := strings.Split(value, ":")
	tag, err := hex.DecodeString(tokens[1])
	if err != nil {
		return "", err
	}

	var (
		iv         []byte
		cipherData []byte
	)

	switch version {
	case cryptoV1:
		if len(tokens[0]) < 12 {
			return "", fmt.Errorf("invalid encrypted value")
		}
		iv = []byte(tokens[0][:12])
		cipherData, err = hex.DecodeString(tokens[0][12:])
	case cryptoV2:
		if len(tokens[0]) < 24 {
			return "", fmt.Errorf("invalid encrypted value")
		}
		iv, err = hex.DecodeString(tokens[0][:24])
		if err != nil {
			return "", err
		}
		cipherData, err = hex.DecodeString(tokens[0][24:])
	default:
		return "", fmt.Errorf("unsupported SFDX crypto version")
	}
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCMWithNonceSize(block, len(iv))
	if err != nil {
		return "", err
	}

	combined := make([]byte, 0, len(cipherData)+len(tag))
	combined = append(combined, cipherData...)
	combined = append(combined, tag...)

	plaintext, err := gcm.Open(nil, iv, combined, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func GetSFDXAuth(user string) (SFDXAuth, error) {
	stateDirs := sfdxStateDirs()
	if len(stateDirs) == 0 {
		return SFDXAuth{}, fmt.Errorf("unable to determine SFDX state directories")
	}

	aliases := loadSFDXAliases(stateDirs)
	username, alias := resolveSFDXUsername(user, aliases)
	if username == "" {
		return SFDXAuth{}, fmt.Errorf("could not resolve SFDX username for %s", user)
	}

	authFile, stateDir, err := loadSFDXAuthFile(username, stateDirs)
	if err != nil {
		return SFDXAuth{}, err
	}

	needsKey := isEncryptedFormat(authFile.AccessToken) ||
		isEncryptedFormat(authFile.RefreshToken) ||
		isEncryptedFormat(authFile.Password)

	var keys []sfdxKey
	if needsKey {
		keys, err = loadSFDXKeys(stateDir)
		if err != nil {
			return SFDXAuth{}, err
		}
	}

	keyIndex := -1
	accessToken, keyIndex, err := decryptWithCandidates(authFile.AccessToken, keyIndex, keys)
	if err != nil {
		return SFDXAuth{}, err
	}
	refreshToken, keyIndex, err := decryptWithCandidates(authFile.RefreshToken, keyIndex, keys)
	if err != nil {
		return SFDXAuth{}, err
	}
	password, keyIndex, err := decryptWithCandidates(authFile.Password, keyIndex, keys)
	if err != nil {
		return SFDXAuth{}, err
	}

	resolvedAlias := alias
	if resolvedAlias == "" {
		resolvedAlias = firstNonEmpty(authFile.Alias, findAliasForUsername(aliases, authFile.Username))
	}

	instanceURL := firstNonEmpty(authFile.InstanceUrl, authFile.LoginUrl)
	if instanceURL == "" {
		return SFDXAuth{}, fmt.Errorf("SFDX auth for %s missing instance URL", username)
	}

	orgID := firstNonEmpty(authFile.OrgId, authFile.Id)
	if orgID == "" {
		return SFDXAuth{}, fmt.Errorf("SFDX auth for %s missing org id", username)
	}

	auth := SFDXAuth{
		AccessToken:        firstNonEmpty(accessToken, authFile.AccessToken),
		Alias:              resolvedAlias,
		ClientId:           authFile.ClientId,
		CreatedBy:          "",
		DevHubId:           "",
		Edition:            "",
		Id:                 orgID,
		InstanceUrl:        instanceURL,
		OrgName:            "",
		Password:           firstNonEmpty(password, authFile.Password),
		Status:             "",
		Username:           firstNonEmpty(authFile.Username, username),
		LoginUrl:           authFile.LoginUrl,
		RefreshToken:       firstNonEmpty(refreshToken, authFile.RefreshToken),
		UserId:             authFile.UserId,
		InstanceApiVersion: authFile.InstanceApiVersion,
	}

	if auth.AccessToken == "" {
		return SFDXAuth{}, fmt.Errorf("SFDX auth for %s missing access token", username)
	}

	return auth, nil
}

func UseSFDXSession(authData SFDXAuth) {
	creds := SFDXAuthToForceSession(authData)
	ForceSaveLogin(creds, os.Stderr)
}

func SFDXAuthToForceSession(auth SFDXAuth) ForceSession {
	apiVersion := auth.InstanceApiVersion
	if apiVersion == "" {
		apiVersion = ApiVersionNumber()
	}

	alias := auth.Alias
	if alias == "" {
		alias = auth.Username
	}

	endpoint := firstNonEmpty(auth.InstanceUrl, auth.LoginUrl)
	if endpoint == "" {
		endpoint = auth.InstanceUrl
	}

	userInfo := &UserInfo{
		UserName: auth.Username,
		OrgId:    auth.Id,
		UserId:   auth.UserId,
	}
	if userInfo.UserName == "" {
		userInfo.UserName = alias
	}

	session := ForceSession{
		AccessToken:  auth.AccessToken,
		InstanceUrl:  auth.InstanceUrl,
		EndpointUrl:  endpoint,
		ClientId:     auth.ClientId,
		RefreshToken: auth.RefreshToken,
		Id:           auth.Id,
		UserInfo:     userInfo,
		SessionOptions: &SessionOptions{
			ApiVersion:    apiVersion,
			Alias:         alias,
			RefreshMethod: RefreshSFDX,
		},
	}
	if session.InstanceUrl == "" {
		session.InstanceUrl = endpoint
	}
	return session
}

var sfdxOrgFilePattern = regexp.MustCompile(`^[^.].+\.json$`)

func ListSFDXAuths() ([]SFDXAuth, error) {
	usernames := listSFDXUsernames()
	if len(usernames) == 0 {
		return nil, nil
	}
	results := make([]SFDXAuth, 0, len(usernames))
	seen := make(map[string]struct{})
	var lastErr error
	for _, username := range usernames {
		auth, err := GetSFDXAuth(username)
		if err != nil {
			lastErr = err
			continue
		}
		key := strings.TrimSpace(auth.Username)
		if key == "" {
			key = strings.TrimSpace(auth.Id)
		}
		if key != "" {
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
		}
		results = append(results, auth)
	}
	if len(results) == 0 && lastErr != nil {
		return nil, lastErr
	}
	sort.Slice(results, func(i, j int) bool {
		left := strings.ToLower(firstNonEmpty(results[i].Alias, results[i].Username, results[i].Id))
		right := strings.ToLower(firstNonEmpty(results[j].Alias, results[j].Username, results[j].Id))
		return left < right
	})
	return results, nil
}

func listSFDXUsernames() []string {
	usernames := map[string]struct{}{}
	for _, dir := range sfdxStateDirs() {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		collectSFDXUsernames(dir, usernames)
	}
	if len(usernames) == 0 {
		return nil
	}
	list := make([]string, 0, len(usernames))
	for username := range usernames {
		list = append(list, username)
	}
	sort.Strings(list)
	return list
}

func collectSFDXUsernames(root string, usernames map[string]struct{}) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			if entry.Name() == "orgs" {
				collectSFDXUsernames(filepath.Join(root, entry.Name()), usernames)
			}
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".json") || !sfdxOrgFilePattern.MatchString(entry.Name()) {
			continue
		}
		path := filepath.Join(root, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var auth sfdxAuthFile
		if err := json.Unmarshal(data, &auth); err != nil {
			continue
		}
		username := strings.TrimSpace(auth.Username)
		if username == "" {
			username = strings.TrimSuffix(entry.Name(), ".json")
		}
		username = strings.TrimSpace(username)
		if username == "" {
			continue
		}
		if _, ok := usernames[username]; !ok {
			usernames[username] = struct{}{}
		}
	}
}

func CompleteSFDXSession(creds *ForceSession, aliasHint string) error {
	if creds == nil {
		return fmt.Errorf("nil force session")
	}
	aliasHint = strings.TrimSpace(aliasHint)

	if strings.TrimSpace(creds.EndpointUrl) == "" {
		creds.EndpointUrl = firstNonEmpty(creds.InstanceUrl, creds.EndpointUrl)
	}

	if creds.SessionOptions == nil {
		creds.SessionOptions = &SessionOptions{}
	}
	if creds.SessionOptions.ApiVersion == "" {
		creds.SessionOptions.ApiVersion = ApiVersionNumber()
	}
	creds.SessionOptions.RefreshMethod = RefreshSFDX

	needsUserInfo := creds.UserInfo == nil || creds.UserInfo.UserId == "" || creds.UserInfo.OrgId == ""
	if needsUserInfo {
		userInfo, err := getUserInfoFn(creds)
		if err != nil {
			return fmt.Errorf("failed to load user info: %w", err)
		}
		creds.UserInfo = &userInfo
	}

	if creds.UserInfo != nil {
		if strings.TrimSpace(creds.UserInfo.OrgId) == "" {
			creds.UserInfo.OrgId = creds.Id
		}
		if strings.TrimSpace(creds.UserInfo.UserName) == "" && aliasHint != "" {
			creds.UserInfo.UserName = aliasHint
		}
	}

	if strings.TrimSpace(creds.Id) == "" && creds.UserInfo != nil {
		creds.Id = creds.UserInfo.OrgId
	}

	userNameHint := ""
	if creds.UserInfo != nil {
		userNameHint = creds.UserInfo.UserName
	}

	if strings.TrimSpace(creds.SessionOptions.Alias) == "" {
		creds.SessionOptions.Alias = firstNonEmpty(aliasHint, userNameHint, creds.Id)
	}

	return nil
}
