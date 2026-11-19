package sf

import (
	"bufio"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"golang.org/x/term"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ForceCLI/force/lib"
)

// scratchDefinitionKeys maps the keys of a scratch org definition file to
// ScratchOrgInfo fields.  "features" and "settings" are handled separately.
var scratchDefinitionKeys = map[string]string{
	"orgName": "OrgName", "edition": "Edition", "country": "Country", "language": "Language",
	"adminEmail": "AdminEmail", "description": "Description", "hasSampleData": "HasSampleData",
	"username": "Username", "release": "Release", "namespace": "Namespace", "duration": "DurationDays",
	"durationDays": "DurationDays", "sourceOrg": "SourceOrg", "snapshot": "Snapshot",
}

// xmlFromJSON writes a JSON value as XML elements: an object becomes child
// elements, an array repeats the element, and other values become text.
func xmlFromJSON(b *bytes.Buffer, name string, value interface{}, indent string) {
	switch v := value.(type) {
	case *orderedMap:
		fmt.Fprintf(b, "%s<%s>\n", indent, name)
		for _, key := range v.keys {
			xmlFromJSON(b, key, v.values[key], indent+"    ")
		}
		fmt.Fprintf(b, "%s</%s>\n", indent, name)
	case []interface{}:
		for _, item := range v {
			xmlFromJSON(b, name, item, indent)
		}
	default:
		fmt.Fprintf(b, "%s<%s>", indent, name)
		xml.EscapeText(b, []byte(displayValue(v)))
		fmt.Fprintf(b, "</%s>\n", name)
	}
}

// scratchSettingsFiles converts the "settings" object of a definition file
// to Settings metadata.  The key lightningExperienceSettings becomes the
// file settings/LightningExperience.settings.
func scratchSettingsFiles(settings *orderedMap) lib.ForceMetadataFiles {
	files := lib.ForceMetadataFiles{}
	var members []string
	for _, key := range settings.keys {
		body, ok := settings.values[key].(*orderedMap)
		if !ok {
			continue
		}
		root := strings.ToUpper(key[:1]) + key[1:]
		member := strings.TrimSuffix(root, "Settings")
		var b bytes.Buffer
		b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
		fmt.Fprintf(&b, "<%s xmlns=\"http://soap.sforce.com/2006/04/metadata\">\n", root)
		for _, field := range body.keys {
			xmlFromJSON(&b, field, body.values[field], "    ")
		}
		fmt.Fprintf(&b, "</%s>\n", root)
		files["settings/"+member+".settings"] = b.Bytes()
		members = append(members, member)
	}
	if len(members) > 0 {
		sort.Strings(members)
		files["package.xml"] = packageXML(map[string][]string{"Settings": members}, lib.ApiVersionNumber())
	}
	return files
}

func newOrgCreateScratchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "scratch",
		Short:       "Create a scratch org.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "OrgCreateScratch"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := sandboxFlagConflicts(cmd, [][2]string{{"edition", "snapshot"}, {"edition", "source-org"}, {"snapshot", "source-org"}}); err != nil {
				return err
			}
			definitionFile, _ := cmd.Flags().GetString("definition-file")
			edition, _ := cmd.Flags().GetString("edition")
			snapshot, _ := cmd.Flags().GetString("snapshot")
			sourceOrg, _ := cmd.Flags().GetString("source-org")
			if sourceOrg != "" {
				if len(sourceOrg) != 15 {
					return flagParseError("InvalidIdLengthError", "source-org", "The id must be 15 characters.")
				}
				if !strings.HasPrefix(sourceOrg, "00D") {
					return flagParseError("InvalidPrefixError", "source-org", "The id must begin with 00D.")
				}
			}
			if definitionFile == "" && edition == "" && snapshot == "" && sourceOrg == "" {
				return &Error{Message: "The following error occurred:\n  Exactly one of the following must be provided: --definition-file, --edition, --snapshot, --source-org\nSee more help with --help", ExitCode: 2}
			}
			request := newOrderedMap()
			var settings *orderedMap
			// definitionJSON is what sf records in its cache for a
			// request: the definition file and the edition flag.
			definitionJSON := newOrderedMap()
			if definitionFile != "" {
				data, err := os.ReadFile(definitionFile)
				if err != nil {
					return err
				}
				parsed, err := parseOrderedJSON(data)
				if err != nil {
					return fmt.Errorf("%s: %w", definitionFile, err)
				}
				definition, ok := parsed.(*orderedMap)
				if !ok {
					return fmt.Errorf("%s is not a JSON object", definitionFile)
				}
				for _, key := range definition.keys {
					value := definition.values[key]
					definitionJSON.Set(key, value)
					switch key {
					case "features":
						var features []string
						switch v := value.(type) {
						case []interface{}:
							for _, feature := range v {
								features = append(features, displayValue(feature))
							}
						case string:
							features = strings.FieldsFunc(v, func(r rune) bool { return r == ';' || r == ',' })
						}
						request.Set("Features", strings.Join(features, ";"))
					case "settings":
						settings, _ = value.(*orderedMap)
					case "objectSettings":
						return newError("UnsupportedDefinitionKey", "The scratch org definition key \"%s\" is not supported by the sf-compatible mode of force.", key)
					default:
						field, known := scratchDefinitionKeys[key]
						if !known {
							return newError("UnsupportedDefinitionKey", "The scratch org definition key \"%s\" is not supported by the sf-compatible mode of force.", key)
						}
						request.Set(field, value)
					}
				}
			}
			flagValues := map[string]string{"edition": "Edition", "username": "Username", "description": "Description", "name": "OrgName", "release": "Release", "admin-email": "AdminEmail"}
			for _, flag := range []string{"edition", "username", "description", "name", "release", "admin-email"} {
				if value, _ := cmd.Flags().GetString(flag); value != "" {
					request.Set(flagValues[flag], value)
				}
			}
			if edition != "" {
				definitionJSON.Set("edition", edition)
			}
			if sourceOrg != "" {
				request.Set("SourceOrg", sourceOrg)
				definitionJSON.Set("sourceOrg", sourceOrg)
			}
			if snapshot != "" {
				request.Set("Snapshot", snapshot)
				definitionJSON.Set("snapshot", snapshot)
			}
			if _, fromSnapshot := definitionJSON.Get("snapshot"); fromSnapshot {
				// A snapshot fixes these properties of the org.
				var invalid []string
				for _, key := range []string{"features", "orgPreferences", "edition", "sourceOrg", "settingsPath", "releaseVersion"} {
					if _, present := definitionJSON.Get(key); present {
						invalid = append(invalid, key)
					}
				}
				if len(invalid) > 0 {
					return newError("orgSnapshot", "Org snapshots don’t support one or more options you specified: %s", strings.Join(invalid, ", "))
				}
			}
			if release, ok := request.Get("Release"); ok {
				release := strings.ToLower(displayValue(release))
				if release != "preview" && release != "previous" {
					return &Error{Message: fmt.Sprintf("Expected --release=%s to be one of: preview, previous\nSee more help with --help", release), ExitCode: 1}
				}
				request.Set("Release", strings.ToUpper(release[:1])+release[1:])
			}
			if _, ok := request.Get("OrgName"); !ok {
				request.Set("OrgName", "Company")
			}
			if editionValue, ok := request.Get("Edition"); ok {
				// The picklist values are capitalized: Developer, Enterprise.
				name := displayValue(editionValue)
				if name != "" {
					words := strings.Split(name, "-")
					for i, word := range words {
						if word != "" {
							words[i] = strings.ToUpper(word[:1]) + word[1:]
						}
					}
					request.Set("Edition", strings.Join(words, " "))
				}
			}
			days, _ := cmd.Flags().GetInt("duration-days")
			if _, fromFile := request.Get("DurationDays"); !fromFile || cmd.Flags().Changed("duration-days") {
				request.Set("DurationDays", days)
			}
			noNamespace, _ := cmd.Flags().GetBool("no-namespace")
			if noNamespace {
				request.Delete("Namespace")
			} else if _, ok := request.Get("Namespace"); !ok {
				if wd, err := os.Getwd(); err == nil {
					if root, found := lib.SFProjectRoot(wd); found {
						if data, err := os.ReadFile(root + "/sfdx-project.json"); err == nil {
							var project struct {
								Namespace string `json:"namespace"`
							}
							if json.Unmarshal(data, &project) == nil && project.Namespace != "" {
								request.Set("Namespace", project.Namespace)
							}
						}
					}
				}
			}
			// sf asks for the secret of the connected app when one is
			// named, before it does anything else.
			client := lib.DefaultScratchOrgClient
			if clientID := flagText(cmd, "client-id"); clientID != "" {
				secret, err := promptSecret(cmd, "OAuth client secret of your personal connected app")
				if err != nil {
					return err
				}
				client = scratchOrgClient(clientID, secret)
			}
			request.Set("ConnectedAppConsumerKey", client.ClientId)
			request.Set("ConnectedAppCallbackUrl", client.RedirectURI)

			hub, err := resolveTargetDevHub(cmd)
			if err != nil {
				return err
			}
			// The ancestors of the packages in sfdx-project.json are
			// installed in the scratch org, unless a flag says not to.
			noAncestors, _ := cmd.Flags().GetBool("no-ancestors")
			if !noNamespace && !noAncestors {
				ancestors, err := packageAncestorIDs(hub)
				if err != nil {
					return err
				}
				if ancestors != "" {
					request.Set("Package2AncestorIds", ancestors)
				}
			}
			started := time.Now()
			content, err := encodeJSON(request)
			if err != nil {
				return err
			}
			body, err := hub.Force.PostREST("/sobjects/ScratchOrgInfo", content)
			if err != nil && strings.Contains(err.Error(), "The callback URL provided is not valid") && client.RedirectURI != lib.RedirectUri {
				// The Dev Hub checks the callback URL against the connected
				// app.  An app that was set up for force has the callback
				// URL of force and not the one sf names, so try that one.
				first := client.RedirectURI
				client.RedirectURI = lib.RedirectUri
				request.Set("ConnectedAppCallbackUrl", client.RedirectURI)
				if content, err = encodeJSON(request); err != nil {
					return err
				}
				if body, err = hub.Force.PostREST("/sobjects/ScratchOrgInfo", content); err != nil {
					converted := apiError(err)
					if failure, ok := converted.(*Error); ok && strings.Contains(failure.Message, "The callback URL provided is not valid") {
						failure.Actions = []string{fmt.Sprintf("Add %s or %s to the callback URLs of the connected app %s. The Dev Hub accepts a scratch org request only with a callback URL of the connected app, and these are the two this command sends.", first, lib.RedirectUri, client.ClientId)}
					}
					return converted
				}
			}
			if err != nil {
				return apiError(err)
			}
			var created struct {
				Id string `json:"id"`
			}
			if err := json.Unmarshal([]byte(body), &created); err != nil || created.Id == "" {
				return fmt.Errorf("the Dev Hub did not return a ScratchOrgInfo Id")
			}
			alias, _ := cmd.Flags().GetString("alias")
			setDefault, _ := cmd.Flags().GetBool("set-default")
			tracksSource, _ := cmd.Flags().GetBool("track-source")
			requestLine := func(id string) [2]string {
				return [2]string{"Request Id", fmt.Sprintf("%s (%s/%s)", id, strings.TrimRight(hub.Fields.String("instanceUrl"), "/"), id)}
			}
			if async, _ := cmd.Flags().GetBool("async"); async {
				// Record the request so that "org resume scratch" can
				// finish it, and report the request without waiting.
				entry := newOrderedMap()
				entry.Set("hubUsername", hub.Username)
				entry.Set("hubBaseUrl", strings.TrimRight(hub.Fields.String("instanceUrl"), "/"))
				entry.Set("definitionjson", definitionJSON)
				if alias != "" {
					entry.Set("alias", alias)
				}
				entry.Set("setDefault", setDefault)
				entry.Set("tracksSource", tracksSource)
				if client.ClientSecret != "" {
					entry.Set("clientSecret", client.ClientSecret)
				}
				if err := writeScratchCacheEntry(created.Id, entry); err != nil {
					return err
				}
				info, err := scratchOrgRequest(hub, created.Id)
				if err != nil {
					return err
				}
				result := newOrderedMap()
				result.Set("username", info.values["SignupUsername"])
				result.Set("scratchOrgInfo", info)
				result.Set("warnings", []interface{}{})
				warnJSON(cmd, secretsHiddenWarning("org create scratch --json"))
				warn(cmd, recordTypeWarning)
				return emit(cmd, result, func(w io.Writer) {
					elapsed := time.Since(started)
					infoLines := [][2]string{requestLine(created.Id), {"OrgId", text(info.values["ScratchOrg"])}, {"Username", text(info.values["SignupUsername"])}}
					if alias != "" {
						infoLines = append(infoLines, [2]string{"Alias", alias})
					}
					infoLines = append(infoLines, [2]string{"Elapsed Time", formatElapsed(elapsed)})
					writeStageFrame(w, "Creating Scratch Org (async)", "", false, []stage{
						{label: "Prepare Request", state: "done"},
						{label: "Send Request", state: "done", elapsed: elapsed},
						{label: "Done", state: "done"},
					}, infoLines)
					fmt.Fprintf(w, "Resume scratch org creation by running \""+binName+" org resume scratch --job-id %s\"\n", created.Id)
				})
			}
			done, err := completeScratchOrg(hub, created.Id, settings, alias, setDefault, tracksSource, client)
			if err != nil {
				return err
			}
			warnJSON(cmd, secretsHiddenWarning("org create scratch --json"))
			warn(cmd, recordTypeWarning)
			return emit(cmd, done.result, func(w io.Writer) {
				elapsed := time.Since(started)
				stages := []stage{
					{label: "Prepare Request", state: "done"},
					{label: "Send Request", state: "done"},
					{label: "Wait For Org", state: "done", elapsed: elapsed},
					{label: "Available", state: "done"},
					{label: "Authenticate", state: "done"},
					{label: "Deploy Settings", state: done.settingsState},
					{label: "Done", state: "done"},
				}
				infoLines := [][2]string{requestLine(created.Id), {"OrgId", text(done.info.values["ScratchOrg"])}, {"Username", done.username}}
				if alias != "" {
					infoLines = append(infoLines, [2]string{"Alias", alias})
				}
				infoLines = append(infoLines, [2]string{"Elapsed Time", formatElapsed(elapsed)})
				writeStageFrame(w, "Creating Scratch Org", "", false, stages, infoLines)
				fmt.Fprintln(w, paint(w, styleGreen, "Your scratch org is ready."))
			})
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().StringP("alias", "a", "", "Alias for the scratch org.")
	cmd.Flags().Bool("async", false, "Request the org, but don't wait for it to complete.")
	cmd.Flags().BoolP("set-default", "d", false, "Set the scratch org as your default org")
	cmd.Flags().StringP("definition-file", "f", "", "Path to a scratch org definition file.")
	cmd.Flags().BoolP("no-ancestors", "c", false, "Don't include second-generation managed package (2GP) ancestors in the scratch org.")
	cmd.Flags().StringP("edition", "e", "", "Salesforce edition of the scratch org. Overrides the value of the \"edition\" option in the definition file, if set.")
	cmd.Flags().BoolP("no-namespace", "m", false, "Create the scratch org with no namespace, even if the Dev Hub has a namespace.")
	cmd.Flags().IntP("duration-days", "y", 7, "Number of days before the org expires.")
	cmd.Flags().IntP("wait", "w", 5, "Number of minutes to wait for the scratch org to be ready.")
	cmd.Flags().StringP("client-id", "i", "", "Consumer key of the Dev Hub connected app.")
	cmd.Flags().BoolP("track-source", "t", true, "Use source tracking for this scratch org.")
	cmd.Flags().String("username", "", "Username of the scratch org admin user.")
	cmd.Flags().String("description", "", "Description of the scratch org in the Dev Hub.")
	cmd.Flags().String("name", "", "Name of the org, such as \"Acme Company\".")
	cmd.Flags().String("release", "", "Release of the scratch org as compared to the Dev Hub release.")
	cmd.Flags().String("admin-email", "", "Email address that will be applied to the org's admin user.")
	cmd.Flags().String("source-org", "", "15-character ID of the org whose shape the new scratch org will be based on.")
	cmd.Flags().StringP("snapshot", "s", "", "Name of the snapshot to use when creating this scratch org.")
	addAPIVersionFlag(cmd)
	addTargetDevHubFlag(cmd)
	return cmd
}

// scratchOrgRequest reads a ScratchOrgInfo record of a Dev Hub, without the
// authorization code.
func scratchOrgRequest(hub *targetOrg, id string) (*orderedMap, error) {
	body, err := hub.Force.GetREST("/sobjects/ScratchOrgInfo/" + id)
	if err != nil {
		return nil, apiError(err)
	}
	parsed, err := parseOrderedJSON([]byte(body))
	if err != nil {
		return nil, err
	}
	info, _ := parsed.(*orderedMap)
	if info == nil {
		info = newOrderedMap()
	}
	info.Delete("AuthCode")
	return info, nil
}

// scratchCachePath returns ~/.sf/scratch-create-cache.json, where sf
// records the scratch org requests it has not finished.
func scratchCachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".sf", "scratch-create-cache.json"), nil
}

func readScratchCache() (*orderedMap, error) {
	path, err := scratchCachePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return newOrderedMap(), nil
		}
		return nil, err
	}
	parsed, err := parseOrderedJSON(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	cache, _ := parsed.(*orderedMap)
	if cache == nil {
		cache = newOrderedMap()
	}
	return cache, nil
}

func writeScratchCache(cache *orderedMap) error {
	path, err := scratchCachePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(cache); err != nil {
		return err
	}
	return os.WriteFile(path, bytes.TrimRight(buffer.Bytes(), "\n"), 0o600)
}

// writeScratchCacheEntry records a scratch org request with the time of
// the request.
func writeScratchCacheEntry(id string, entry *orderedMap) error {
	cache, err := readScratchCache()
	if err != nil {
		return err
	}
	entry.Set("timestamp", time.Now().UTC().Format("2006-01-02T15:04:05.000Z"))
	cache.Set(id, entry)
	return writeScratchCache(cache)
}

// scratchOrgClient returns the connected app to request a scratch org with.
// The callback URL is the one sf names, except for the connected app of
// force, which has a callback URL of its own.
func scratchOrgClient(clientID, secret string) lib.ScratchOrgClient {
	client := lib.ScratchOrgClient{ClientId: clientID, ClientSecret: secret, RedirectURI: lib.DefaultScratchOrgClient.RedirectURI}
	if clientID == lib.ClientId {
		client.RedirectURI = lib.RedirectUri
	}
	return client
}

// promptSecret asks for a secret on stderr and reads it without echo from a
// terminal, or as a line from other input.
func promptSecret(cmd *cobra.Command, message string) (string, error) {
	fmt.Fprintf(cmd.ErrOrStderr(), "? %s ", message)
	if file, ok := cmd.InOrStdin().(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		secret, err := term.ReadPassword(int(file.Fd()))
		fmt.Fprintln(cmd.ErrOrStderr())
		return string(secret), err
	}
	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	fmt.Fprintln(cmd.ErrOrStderr())
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// completedScratchOrg is a scratch org that is authorized and set up.
type completedScratchOrg struct {
	username      string
	info          *orderedMap
	result        *orderedMap
	settingsState string
}

// completeScratchOrg finishes a scratch org request: it waits for the org,
// authorizes it with the authorization code of the request, writes the
// authorization file, sets the alias and the default org, and deploys the
// settings of the definition.
func completeScratchOrg(hub *targetOrg, id string, settings *orderedMap, alias string, setDefault, tracksSource bool, client lib.ScratchOrgClient) (*completedScratchOrg, error) {
	// sf prints nothing while it waits for the org and logs in to it.
	previous := lib.ScratchOrgProgress
	lib.ScratchOrgProgress = io.Discard
	defer func() { lib.ScratchOrgProgress = previous }()
	session, err := hub.Force.ForceLoginNewScratchWithClient(id, client)
	if err != nil {
		return nil, &Error{
			Name:     "ScratchOrgCreateError",
			Message:  err.Error(),
			ExitCode: 1,
			Actions:  []string{fmt.Sprintf("Resume scratch org creation by running \""+binName+" org resume scratch --job-id %s\"", id)},
		}
	}
	info, err := scratchOrgRequest(hub, id)
	if err != nil {
		return nil, err
	}
	// The alias and the default org are set below; the command given to
	// saveAuthorization has no flags for them.
	fields, order, err := saveAuthorization(&cobra.Command{}, authorization{
		session:  session,
		loginURL: session.InstanceUrl,
		order:    []string{"accessToken", "instanceUrl", "orgId", "username", "loginUrl", "refreshToken", "clientId"},
	})
	if err != nil {
		return nil, err
	}
	username := fields.String("username")
	extra := lib.SFDXAuthFields{
		"devHubUsername": hub.Username,
		"isScratch":      true,
		"isDevHub":       false,
		"created":        fmt.Sprintf("%d", time.Now().UnixMilli()),
		"tracksSource":   tracksSource,
	}
	if client.ClientSecret != "" {
		extra["clientSecret"] = client.ClientSecret
	}
	if expiration := text(info.values["ExpirationDate"]); expiration != "" {
		extra["expirationDate"] = expiration
	}
	if instance := text(info.values["SignupInstance"]); instance != "" {
		extra["createdOrgInstance"] = instance
	}
	if err := lib.UpdateSFDXAuthFields(username, extra); err != nil {
		return nil, err
	}
	fields, order, err = lib.ReadSFDXAuthFieldsOrdered(username)
	if err != nil {
		return nil, err
	}
	target := username
	if alias != "" {
		if err := lib.SetSFDXAlias(alias, username); err != nil {
			return nil, err
		}
		target = alias
	}
	if setDefault {
		_, inProject := lib.SFLocalConfigPath()
		if _, err := lib.SetSFConfig(lib.SFConfigTargetOrg, target, !inProject); err != nil {
			return nil, err
		}
	}

	settingsState := "skipped"
	if settings != nil {
		files := scratchSettingsFiles(settings)
		if len(files) > 0 {
			scratch, err := loadOrg(username)
			if err != nil {
				return nil, err
			}
			if version := fields.String("instanceApiVersion"); version != "" {
				lib.SetApiVersion(version)
				scratch.Force.Metadata = lib.NewForceMetadata(scratch.Force)
			}
			result, err := scratch.Force.Metadata.Deploy(files, lib.ForceDeployOptions{RollbackOnError: true, SinglePackage: true})
			if err != nil {
				return nil, apiError(err)
			}
			if !result.Success {
				var problems []string
				for _, failure := range result.Details.ComponentFailures {
					problems = append(problems, fmt.Sprintf("%s: %s", failure.FullName, failure.Problem))
				}
				return nil, newError("SettingsDeployError", "A scratch org was created with username %s, but the settings failed to deploy due to: \n%s", username, strings.Join(problems, "\n"))
			}
			settingsState = "done"
		}
	}

	authFields := newOrderedMap()
	for _, key := range order {
		if secretFields[key] {
			continue
		}
		value := fields[key]
		if key == "accessToken" {
			value = redactedSecret()
		}
		authFields.Set(key, value)
	}
	result := newOrderedMap()
	result.Set("username", username)
	result.Set("scratchOrgInfo", info)
	result.Set("authFields", authFields)
	result.Set("warnings", []interface{}{})
	result.Set("orgId", fields.String("orgId"))
	return &completedScratchOrg{username: username, info: info, result: result, settingsState: settingsState}, nil
}

// recordTypeWarning is the warning sf gives with every scratch org request.
const recordTypeWarning = "Record types defined in the scratch org definition file will stop being capitalized by default in a future release.\nSet the `org-capitalize-record-types` config var to `true` to enforce capitalization."

// scratchOrgStatusWait is the time between checks of a scratch org request
// that is not finished.  Tests shorten it.
var scratchOrgStatusWait = 5 * time.Second

func newOrgResumeScratchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "scratch",
		Short:       "Resume the creation of an incomplete scratch org.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "OrgResumeScratch"},
		RunE: func(cmd *cobra.Command, args []string) error {
			id, _ := cmd.Flags().GetString("job-id")
			mostRecent, _ := cmd.Flags().GetBool("use-most-recent")
			if (id == "") == !mostRecent {
				return exactlyOneError("use-most-recent", "job-id")
			}
			if id != "" {
				if err := salesforceIDFlag("job-id", id, "2SR", true); err != nil {
					return err
				}
			}
			cache, err := readScratchCache()
			if err != nil {
				return err
			}
			if mostRecent {
				newest := ""
				for _, key := range cache.keys {
					entry, _ := cache.values[key].(*orderedMap)
					if entry == nil {
						continue
					}
					if stamp := text(entry.values["timestamp"]); id == "" || stamp > newest {
						id, newest = key, stamp
					}
				}
				if id == "" {
					return newError("NoRecentJobIdError", "There are no recent job IDs (ScratchOrgInfo requests) in your cache. Maybe it completed or already resumed?")
				}
			}
			started := time.Now()
			out := cmd.OutOrStdout()
			labels := []string{"Prepare Request", "Send Request", "Wait For Org", "Available", "Authenticate", "Deploy Settings", "Done"}
			failedFrame := func() {
				if jsonRequested(cmd) {
					return
				}
				stages := []stage{{label: labels[0], state: "skipped"}, {label: labels[1], state: "failed", elapsed: time.Since(started)}}
				for _, label := range labels[2:] {
					stages = append(stages, stage{label: label})
				}
				writeStageFrame(out, "Resuming Scratch Org", "", false, stages, [][2]string{{"Request Id", "✘"}, {"OrgId", "✘"}, {"Username", "✘"}, {"Elapsed Time", formatElapsed(time.Since(started))}})
			}
			entry, _ := cache.values[id].(*orderedMap)
			if entry == nil {
				failedFrame()
				// sf reports this error also when the cache is empty.
				return newError("JobIdMismatchError", "There are no recent job IDs (ScratchOrgInfo requests) in your cache that match %s. Maybe it completed?", id)
			}
			hub, err := loadOrg(text(entry.values["hubUsername"]))
			if err != nil {
				return newError("NamedOrgNotFoundError", "No authorization information found for %s.", text(entry.values["hubUsername"]))
			}
			if version := hub.Fields.String("instanceApiVersion"); version != "" {
				lib.SetApiVersion(version)
			}
			// Wait for the org for the time given by --wait.
			minutes, _ := cmd.Flags().GetInt("wait")
			deadline := time.Now().Add(time.Duration(minutes) * time.Minute)
			for {
				info, err := scratchOrgRequest(hub, id)
				if err != nil {
					failedFrame()
					return err
				}
				status := text(info.values["Status"])
				if status == "Active" || status == "Error" {
					// ForceLoginNewScratch reports the error of a failed
					// request.
					break
				}
				if !time.Now().Before(deadline) {
					failedFrame()
					return &Error{
						Name:     "StillInProgressError",
						Message:  fmt.Sprintf("The scratch org isn't ready yet (Status = %s).", status),
						ExitCode: 1,
						Actions:  []string{"Increase the --wait time because creating a scratch org can take a while."},
					}
				}
				time.Sleep(scratchOrgStatusWait)
			}
			var settings *orderedMap
			if definition, ok := entry.values["definitionjson"].(*orderedMap); ok {
				settings, _ = definition.values["settings"].(*orderedMap)
			}
			alias := text(entry.values["alias"])
			setDefault, _ := entry.values["setDefault"].(bool)
			tracksSource := true
			if value, ok := entry.values["tracksSource"].(bool); ok {
				tracksSource = value
			}
			// The request names the connected app; the cache has its
			// secret when it has one.
			client := lib.DefaultScratchOrgClient
			if requested, err := scratchOrgRequest(hub, id); err == nil {
				if key := text(requested.values["ConnectedAppConsumerKey"]); key != "" {
					client = scratchOrgClient(key, text(entry.values["clientSecret"]))
				}
			}
			done, err := completeScratchOrg(hub, id, settings, alias, setDefault, tracksSource, client)
			if err != nil {
				failedFrame()
				return err
			}
			cache.Delete(id)
			if err := writeScratchCache(cache); err != nil {
				return err
			}
			warnJSON(cmd, secretsHiddenWarning("org resume scratch --json"))
			warn(cmd, recordTypeWarning)
			return emit(cmd, done.result, func(w io.Writer) {
				elapsed := time.Since(started)
				stages := []stage{
					{label: "Prepare Request", state: "skipped"},
					{label: "Send Request", state: "done"},
					{label: "Wait For Org", state: "skipped"},
					{label: "Available", state: "done"},
					{label: "Authenticate", state: "done", elapsed: elapsed},
					{label: "Deploy Settings", state: "done"},
					{label: "Done", state: "done"},
				}
				infoLines := [][2]string{
					{"Request Id", fmt.Sprintf("%s (%s/%s)", id, text(entry.values["hubBaseUrl"]), id)},
					{"OrgId", text(done.info.values["ScratchOrg"])},
					{"Username", done.username},
				}
				if alias != "" {
					infoLines = append(infoLines, [2]string{"Alias", alias})
				}
				infoLines = append(infoLines, [2]string{"Elapsed Time", formatElapsed(elapsed)})
				writeStageFrame(w, "Resuming Scratch Org", "", false, stages, infoLines)
				fmt.Fprintln(w)
				fmt.Fprintln(w, paint(w, styleGreen, "Your scratch org is ready."))
			})
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().StringP("job-id", "i", "", "Job ID of the incomplete scratch org create that you want to resume.")
	cmd.Flags().BoolP("use-most-recent", "r", false, "Use the job ID of the most recent incomplete scratch org.")
	cmd.Flags().IntP("wait", "w", 0, "Number of minutes to wait for the scratch org to be ready.")
	return cmd
}

var ancestorVersionPattern = regexp.MustCompile(`^[0-9]+.[0-9]+.[0-9]+(.[0-9]+)?$`)

// packageAncestorIDs returns the Package2Version Ids of the ancestors that
// the package directories of sfdx-project.json name, joined by semicolons,
// as sf sends them with a scratch org request.
func packageAncestorIDs(hub *targetOrg) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", nil
	}
	root, found := lib.SFProjectRoot(wd)
	if !found {
		return "", nil
	}
	data, err := os.ReadFile(filepath.Join(root, "sfdx-project.json"))
	if err != nil {
		return "", nil
	}
	var project struct {
		PackageDirectories []struct {
			Package         string `json:"package"`
			AncestorID      string `json:"ancestorId"`
			AncestorVersion string `json:"ancestorVersion"`
		} `json:"packageDirectories"`
		PackageAliases map[string]string `json:"packageAliases"`
	}
	if json.Unmarshal(data, &project) != nil {
		return "", nil
	}
	notReleased := func(value string) []string {
		return []string{fmt.Sprintf("The ancestor package version [%s] specified in the sfdx-project.json file may exist hasn’t been promoted and released. Release the ancestor package version before specifying it as the ancestor in a new package or patch version.", value)}
	}
	single := func(soql string) (string, bool) {
		records, err := toolingRecords(hub, soql)
		if err != nil || len(records) != 1 {
			return "", false
		}
		return text(records[0].values["Id"]), true
	}
	var ids []string
	seen := map[string]bool{}
	for _, directory := range project.PackageDirectories {
		if directory.Package == "" || (directory.AncestorID == "" && directory.AncestorVersion == "") {
			continue
		}
		packageRecordID := directory.Package
		if id, ok := project.PackageAliases[directory.Package]; ok {
			packageRecordID = id
		}
		id := ""
		switch {
		case directory.AncestorVersion == "NONE" || directory.AncestorID == "NONE":
			continue
		case directory.AncestorVersion == "HIGHEST" || directory.AncestorID == "HIGHEST":
			found, ok := single(fmt.Sprintf("SELECT Id FROM Package2Version WHERE Package2Id = '%s' AND IsReleased = True AND IsDeprecated = False AND PatchVersion = 0 ORDER BY MajorVersion Desc, MinorVersion Desc, PatchVersion Desc, BuildNumber Desc LIMIT 1", packageRecordID))
			if !ok {
				if directory.AncestorVersion == "HIGHEST" {
					return "", &Error{Name: "NoMatchingAncestorError", Message: fmt.Sprintf("The ancestor for ancestorVersion %s can't be found. Package ID %s.", directory.AncestorVersion, directory.Package), ExitCode: 1, Actions: notReleased(directory.AncestorVersion)}
				}
				return "", &Error{Name: "NoMatchingAncestorIdError", Message: fmt.Sprintf("The ancestor for ancestorId [%s] can't be found. Package ID %s.", directory.AncestorID, directory.Package), ExitCode: 1, Actions: notReleased(directory.AncestorID)}
			}
			id = found
		case directory.AncestorVersion != "":
			if !ancestorVersionPattern.MatchString(directory.AncestorVersion) {
				return "", newError("InvalidAncestorVersionFormatError", "The ancestor versionNumber must be in the format major.minor.patch but the value found is %s", directory.AncestorVersion)
			}
			parts := strings.Split(directory.AncestorVersion, ".")
			found, ok := single(fmt.Sprintf("SELECT Id, IsReleased FROM Package2Version WHERE Package2Id = '%s' AND MajorVersion = %s AND MinorVersion = %s AND PatchVersion = %s and IsReleased = true", packageRecordID, parts[0], parts[1], parts[2]))
			if !ok {
				return "", &Error{Name: "NoMatchingAncestorError", Message: fmt.Sprintf("The ancestor for ancestorVersion %s can't be found. Package ID %s.", directory.AncestorVersion, directory.Package), ExitCode: 1, Actions: notReleased(directory.AncestorVersion)}
			}
			if directory.AncestorID != "" && directory.AncestorID != found {
				return "", newError("AncestorIdVersionMismatchError", "The ancestorVersion in sfdx-project.json is not the version expected for the ancestorId you supplied. ancestorVersion %s. ancestorID %s.", directory.AncestorVersion, directory.AncestorID)
			}
			id = found
		case strings.HasPrefix(directory.AncestorID, "05i"):
			id = directory.AncestorID
		case strings.HasPrefix(directory.AncestorID, "04t"):
			found, ok := single(fmt.Sprintf("SELECT Id FROM Package2Version WHERE SubscriberPackageVersionId = '%s'", directory.AncestorID))
			if !ok {
				return "", newError("SingleRecordQuery_NoRecords", "No record found for SELECT Id FROM Package2Version WHERE SubscriberPackageVersionId = '%s'", directory.AncestorID)
			}
			id = found
		default:
			alias, ok := project.PackageAliases[directory.AncestorID]
			if !ok {
				return "", newError("InvalidAncestorId", "Invalid ancestorId %s", directory.AncestorID)
			}
			id = alias
		}
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return strings.Join(ids, ";"), nil
}
