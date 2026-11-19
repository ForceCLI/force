package lib

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// useTempSFHome points the sf state directories at a temporary home
// directory and keeps the encryption key in key.json there.
func useTempSFHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("FORCE_SFDX_STATE_DIRS", "")
	t.Setenv("SF_USE_GENERIC_UNIX_KEYCHAIN", "true")
	// A secret-tool that exits 1 is reported as "no key in the keychain".
	secretTool := filepath.Join(home, "secret-tool")
	if err := os.WriteFile(secretTool, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(secretToolEnvVar, secretTool)
	for _, names := range sfConfigEnvVars {
		for _, name := range names {
			t.Setenv(name, "")
		}
	}
	return home
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(previous) })
}

func TestSFConfigPrecedence(t *testing.T) {
	home := useTempSFHome(t)
	project := filepath.Join(home, "project")
	sub := filepath.Join(project, "force-app", "main")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "sfdx-project.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	chdir(t, sub)

	if _, found, _ := GetSFConfig(SFConfigTargetOrg); found {
		t.Fatal("target-org is set before any config is written")
	}

	globalPath, err := SetSFConfig(SFConfigTargetOrg, "global-org", true)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".sf", "config.json"); globalPath != want {
		t.Errorf("global path = %s, want %s", globalPath, want)
	}
	value, _, _ := GetSFConfig(SFConfigTargetOrg)
	if value.Value != "global-org" || value.Location != SFConfigLocationGlobal {
		t.Errorf("after global set: %+v", value)
	}

	localPath, err := SetSFConfig(SFConfigTargetOrg, "local-org", false)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(project, ".sf", "config.json"); localPath != want {
		t.Errorf("local path = %s, want %s", localPath, want)
	}
	value, _, _ = GetSFConfig(SFConfigTargetOrg)
	if value.Value != "local-org" || value.Location != SFConfigLocationLocal {
		t.Errorf("local value does not replace global value: %+v", value)
	}

	t.Setenv("SF_TARGET_ORG", "env-org")
	value, _, _ = GetSFConfig(SFConfigTargetOrg)
	if value.Value != "env-org" || value.Location != SFConfigLocationEnv {
		t.Errorf("environment value does not replace local value: %+v", value)
	}
	t.Setenv("SF_TARGET_ORG", "")

	if _, err := UnsetSFConfig(SFConfigTargetOrg, false); err != nil {
		t.Fatal(err)
	}
	value, _, _ = GetSFConfig(SFConfigTargetOrg)
	if value.Value != "global-org" {
		t.Errorf("after local unset: %+v", value)
	}
}

func TestSFConfigLocalRequiresProject(t *testing.T) {
	home := useTempSFHome(t)
	chdir(t, home)
	if _, err := SetSFConfig(SFConfigTargetOrg, "x", false); err != ErrNotInSFProject {
		t.Errorf("err = %v, want ErrNotInSFProject", err)
	}
}

func TestSFConfigReadsSfdxConfigFile(t *testing.T) {
	home := useTempSFHome(t)
	chdir(t, home)
	if err := os.MkdirAll(filepath.Join(home, ".sfdx"), 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := `{"defaultdevhubusername": "hub@example.com"}`
	if err := os.WriteFile(filepath.Join(home, ".sfdx", "sfdx-config.json"), []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	value, found, err := GetSFConfig(SFConfigTargetDevHub)
	if err != nil || !found || value.Value != "hub@example.com" {
		t.Errorf("value = %+v, found = %v, err = %v", value, found, err)
	}
}

func TestSFDXAliasesKeepOrder(t *testing.T) {
	home := useTempSFHome(t)
	for _, pair := range [][2]string{{"zeta", "z@example.com"}, {"alpha", "a@example.com"}, {"zeta", "z2@example.com"}} {
		if err := SetSFDXAlias(pair[0], pair[1]); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(filepath.Join(home, ".sfdx", "alias.json"))
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"orgs":{"zeta":"z2@example.com","alpha":"a@example.com"}}`; string(data) != want {
		t.Errorf("alias.json = %s, want %s", data, want)
	}
	if got := ResolveSFDXUsername("alpha"); got != "a@example.com" {
		t.Errorf("ResolveSFDXUsername(alpha) = %s", got)
	}
	if got := ResolveSFDXUsername("other@example.com"); got != "other@example.com" {
		t.Errorf("ResolveSFDXUsername of a username = %s", got)
	}
	value, found, err := UnsetSFDXAlias("zeta")
	if err != nil || !found || value != "z2@example.com" {
		t.Errorf("unset = %q, %v, %v", value, found, err)
	}
	aliases, _ := ListSFDXAliasesOrdered()
	if len(aliases) != 1 || aliases[0].Alias != "alpha" {
		t.Errorf("aliases after unset = %+v", aliases)
	}
}

func TestEncryptSFDXFieldRoundTrip(t *testing.T) {
	v1Key, v1, err := parseSFDXKey("0123456789abcdef0123456789abcdef")
	if err != nil || v1 != cryptoV1 {
		t.Fatalf("v1 key: %v %v", v1, err)
	}
	v2Key, v2, err := parseSFDXKey(strings.Repeat("0123456789abcdef", 4))
	if err != nil || v2 != cryptoV2 {
		t.Fatalf("v2 key: %v %v", v2, err)
	}
	for _, c := range []struct {
		key     []byte
		version sfdxCryptoVersion
	}{{v1Key, v1}, {v2Key, v2}} {
		encrypted, err := encryptSFDXField("00Dxx!token.value", c.key, c.version)
		if err != nil {
			t.Fatal(err)
		}
		if !isEncryptedFormat(encrypted) {
			t.Errorf("version %d: %q is not in the encrypted format", c.version, encrypted)
		}
		decrypted, err := decryptSFDXField(encrypted, c.key, c.version)
		if err != nil || decrypted != "00Dxx!token.value" {
			t.Errorf("version %d: decrypted = %q, err = %v", c.version, decrypted, err)
		}
	}
}

func TestWriteSFDXAuthFieldsEncryptsAndIsReadable(t *testing.T) {
	home := useTempSFHome(t)
	fields := SFDXAuthFields{
		"accessToken":  "00Dxx!access",
		"refreshToken": "5Aep861.refresh",
		"clientSecret": "shh",
		"instanceUrl":  "https://example.my.salesforce.com",
		"loginUrl":     "https://login.salesforce.com",
		"orgId":        "00Dxx0000000001EAA",
		"username":     "me@example.com",
		"clientId":     "PlatformCLI",
		"isDevHub":     true,
	}
	if err := WriteSFDXAuthFields(fields); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".sfdx", "me@example.com.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Errorf("auth file mode = %v, want 0600", info.Mode().Perm())
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"accessToken", "refreshToken", "clientSecret"} {
		stored := raw[name].(string)
		if stored == fields[name] || !isEncryptedFormat(stored) {
			t.Errorf("%s is stored without encryption: %q", name, stored)
		}
	}
	if raw["instanceUrl"] != "https://example.my.salesforce.com" || raw["isDevHub"] != true {
		t.Errorf("plain fields changed: %v", raw)
	}
	if _, err := os.Stat(filepath.Join(home, ".sfdx", "key.json")); err != nil {
		t.Errorf("key.json was not created: %v", err)
	}

	read, err := ReadSFDXAuthFields("me@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if read.String("accessToken") != "00Dxx!access" || read.String("refreshToken") != "5Aep861.refresh" || !read.Bool("isDevHub") {
		t.Errorf("read fields = %v", read)
	}

	// The reader that force already had accepts the file.
	auth, err := GetSFDXAuth("me@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if auth.AccessToken != "00Dxx!access" || auth.RefreshToken != "5Aep861.refresh" || auth.Id != "00Dxx0000000001EAA" {
		t.Errorf("GetSFDXAuth = %+v", auth)
	}

	if err := UpdateSFDXAuthFields("me@example.com", SFDXAuthFields{"accessToken": "00Dxx!second"}); err != nil {
		t.Fatal(err)
	}
	read, _ = ReadSFDXAuthFields("me@example.com")
	if read.String("accessToken") != "00Dxx!second" || read.String("refreshToken") != "5Aep861.refresh" {
		t.Errorf("after update = %v", read)
	}

	if err := DeleteSFDXAuth("me@example.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSFDXAuthFields("me@example.com"); err == nil {
		t.Error("auth file is readable after delete")
	}
}
