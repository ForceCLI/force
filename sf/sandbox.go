package sf

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

// The sandbox commands follow the sf plugin source and the sf core library.
// The output of "org create sandbox --async" and of "org resume sandbox"
// for a pending sandbox was compared with sf.  The login to a finished
// sandbox, the refresh and the deletion were not.

const sandboxProcessFields = "Id,Status,SandboxName,SandboxInfoId,LicenseType,CreatedDate,CopyProgress,SandboxOrganization,SourceId,Description,EndDate,Features"

const sandboxInfoFields = "Id,IsDeleted,CreatedDate,CreatedById,LastModifiedDate,LastModifiedById,SandboxName,LicenseType,TemplateId,HistoryDays,CopyChatter,AutoActivate,ApexClassId,Description,SourceId,ActivationUserGroupId,Features,PostCopyConfig"

// sandboxRedirectURI is the redirect URI sf names when it asks the
// production org to authorize a sandbox user.  It is the callback URL of
// the connected app of sf, PlatformCLI.
const sandboxRedirectURI = "http://localhost:1717/OauthRedirect"

// sandboxClient returns the connected app and the callback URL to name in
// the authorization request for a sandbox.  The production org checks that
// the callback URL belongs to the connected app, so an org that was
// authorized with the connected app of force, as "org login web" of this
// mode does, needs the callback URL of that app.  sf always sends its own
// callback URL, and its request fails for such an org.
func sandboxClient(prod *targetOrg) (clientID, callbackURL string) {
	clientID = prod.Fields.String("clientId")
	switch clientID {
	case "":
		return "PlatformCLI", sandboxRedirectURI
	case lib.ClientId:
		return clientID, lib.RedirectUri
	}
	return clientID, sandboxRedirectURI
}

// sandboxSleep waits between two checks of a sandbox process.  Tests
// replace it.
var sandboxSleep = time.Sleep

// errSandboxTimeout reports that the wait of a sandbox command ended
// before the sandbox was ready.
var errSandboxTimeout = errors.New("The client has timed out.")

var sandboxLicenseTypes = []string{"Developer", "Developer_Pro", "Partial", "Full"}

// sandboxProcesses returns the SandboxProcess records for a condition,
// newest first, without those in a deleting status.
func sandboxProcesses(prod *targetOrg, where string) ([]*orderedMap, string, error) {
	soql := fmt.Sprintf("SELECT %s FROM SandboxProcess WHERE %s ORDER BY CreatedDate DESC", sandboxProcessFields, where)
	records, err := toolingRecords(prod, soql)
	if err != nil {
		return nil, soql, err
	}
	var kept []*orderedMap
	for _, record := range records {
		if !strings.HasPrefix(text(record.values["Status"]), "Del") {
			kept = append(kept, record)
		}
	}
	return kept, soql, nil
}

// sandboxProcess returns the one SandboxProcess record for a condition.
// With several records the error has the name
// SingleRecordQuery_MultipleRecords and the records are returned too.
func sandboxProcess(prod *targetOrg, where string) (*orderedMap, []*orderedMap, error) {
	records, soql, err := sandboxProcesses(prod, where)
	if err != nil {
		return nil, nil, err
	}
	switch len(records) {
	case 0:
		return nil, nil, newError("SingleRecordQuery_NoRecords", "No record found for %s", soql)
	case 1:
		return records[0], nil, nil
	}
	return nil, records, newError("SingleRecordQuery_MultipleRecords", "The query returned more than 1 record")
}

// sandboxAuthorization asks the production org to authorize the user of a
// finished sandbox.  It returns nil while the sandbox process has no end
// date.
func sandboxAuthorization(prod *targetOrg, process *orderedMap) (*orderedMap, error) {
	if process.values["EndDate"] == nil {
		return nil, nil
	}
	clientID, callbackURL := sandboxClient(prod)
	body, err := json.Marshal(orderedFields("clientId", clientID, "sandboxName", process.values["SandboxName"], "callbackUrl", callbackURL))
	if err != nil {
		return nil, err
	}
	response, err := prod.Force.PostREST("/tooling/sandboxAuth", string(body))
	if err != nil {
		converted := apiError(err)
		if e, ok := converted.(*Error); ok && e.Name == "INVALID_STATUS" {
			message := e.Message
			if data, ok := e.Data.(*orderedMap); ok && text(data.values["message"]) != "" {
				message = text(data.values["message"])
			}
			// The end date can be set before the sandbox is ready.
			return nil, newError("SandboxAuthIncompleteError", "The sandbox org completed, but the authentication endpoint is not ready yet (INVALID_STATUS: %s). Run \"org resume sandbox\" to retry.", message)
		}
		return nil, converted
	}
	parsed, err := parseOrderedJSON([]byte(response))
	if err != nil {
		return nil, err
	}
	result, _ := parsed.(*orderedMap)
	return result, nil
}

// writeSandboxAuthorization logs in to a sandbox with the authorization the
// production org gave, and writes the authorization file and the sandbox
// file.  It returns the username of the sandbox.
func writeSandboxAuthorization(prod *targetOrg, process, granted *orderedMap) (string, error) {
	username := text(granted.values["authUserName"])
	if username == "" {
		return "", newError("MissingAuthUsernameError", "The sandbox %s does not have an authorized username.", text(process.values["SandboxName"]))
	}
	loginURL := text(granted.values["loginUrl"])
	auth := authorization{loginURL: loginURL}
	if keyFile := prod.Fields.String("privateKey"); keyFile != "" {
		clientID := prod.Fields.String("clientId")
		assertion, err := lib.JwtAssertionForEndpoint(loginURL, username, keyFile, clientID)
		if err != nil {
			return "", newError("JwtAuthError", "Error authenticating with JWT.\nErrors encountered:\n%s", err.Error())
		}
		session, err := lib.JWTLoginAtEndpoint(loginURL, assertion)
		if err != nil {
			return "", newError("JwtAuthError", "Error authenticating with JWT.\nErrors encountered:\n%s", err.Error())
		}
		session.ClientId = clientID
		auth.session = session
		auth.privateKey = keyFile
		auth.order = []string{"accessToken", "instanceUrl", "orgId", "username", "loginUrl", "privateKey", "clientId"}
	} else {
		clientID, callbackURL := sandboxClient(prod)
		session, err := lib.SessionFromAuthCode(loginURL, text(granted.values["authCode"]), clientID, callbackURL)
		if err != nil {
			return "", newError("AuthCodeExchangeError", "%s", err.Error())
		}
		if instance := text(granted.values["instanceUrl"]); instance != "" && session.InstanceUrl == "" {
			session.InstanceUrl = instance
		}
		auth.session = session
		auth.order = []string{"accessToken", "instanceUrl", "orgId", "username", "loginUrl", "refreshToken", "clientId"}
	}
	// A refreshed sandbox has an authorization file from before the
	// refresh.
	lib.DeleteSFDXAuth(username)
	fields, _, err := saveAuthorization(&cobra.Command{}, auth)
	if err != nil {
		return "", err
	}
	username = fields.String("username")
	if err := lib.UpdateSFDXAuthFields(username, lib.SFDXAuthFields{"isSandbox": true, "isScratch": false}); err != nil {
		return "", err
	}
	orgID := fields.String("orgId")
	if orgID == "" {
		return "", newError("AuthInfoOrgIdUndefinedError", "AuthInfo orgId is undefined.")
	}
	config := sandboxConfig{
		SandboxUsername:  username,
		SandboxOrgID:     orgID,
		ProdOrgUsername:  prod.Username,
		SandboxName:      text(process.values["SandboxName"]),
		SandboxProcessID: text(process.values["Id"]),
		SandboxInfoID:    text(process.values["SandboxInfoId"]),
		Timestamp:        time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
	}
	path, err := sandboxConfigPath(orgID)
	if err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return username, nil
}

func sandboxCachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".sf", "sandbox-create-cache.json"), nil
}

// readSandboxCache reads ~/.sf/sandbox-create-cache.json: one entry for
// each sandbox request that has not finished, by sandbox name.
func readSandboxCache() (*orderedMap, error) {
	path, err := sandboxCachePath()
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
		return newOrderedMap(), nil
	}
	cache, ok := parsed.(*orderedMap)
	if !ok {
		return newOrderedMap(), nil
	}
	return cache, nil
}

func writeSandboxCache(cache *orderedMap) error {
	path, err := sandboxCachePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cache, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// latestSandboxCacheEntry returns the entry with the newest timestamp.
func latestSandboxCacheEntry(cache *orderedMap) (string, *orderedMap) {
	name, newest := "", ""
	var found *orderedMap
	for _, key := range cache.keys {
		entry, ok := cache.values[key].(*orderedMap)
		if !ok {
			continue
		}
		if stamp := text(entry.values["timestamp"]); found == nil || stamp > newest {
			name, newest, found = key, stamp, entry
		}
	}
	return name, found
}

// sandboxRun holds the state of one sandbox command.
type sandboxRun struct {
	cmd          *cobra.Command
	prod         *targetOrg
	action       string // Create or Refresh
	title        string
	refresh      bool
	alias        string
	setDefault   bool
	tracksSource *bool
	// entry is the cache entry of the request.
	entry    *orderedMap
	latest   *orderedMap
	username string
	started  time.Time
	authing  bool
}

// saveEntry writes the cache entry with the latest sandbox process.
func (r *sandboxRun) saveEntry() {
	if r.entry == nil {
		return
	}
	if r.latest != nil {
		r.entry.Set("sandboxProcessObject", r.latest)
	}
	process, _ := r.entry.values["sandboxProcessObject"].(*orderedMap)
	if process == nil || text(process.values["SandboxName"]) == "" {
		return
	}
	cache, err := readSandboxCache()
	if err != nil {
		return
	}
	r.entry.Set("timestamp", time.Now().UTC().Format("2006-01-02T15:04:05.000Z"))
	cache.Set(text(process.values["SandboxName"]), r.entry)
	writeSandboxCache(cache)
}

func (r *sandboxRun) removeEntry() {
	if r.latest == nil || text(r.latest.values["SandboxName"]) == "" {
		return
	}
	cache, err := readSandboxCache()
	if err != nil {
		return
	}
	cache.Delete(text(r.latest.values["SandboxName"]))
	writeSandboxCache(cache)
}

// frame prints the progress display with the running stage in the given
// final state: done, async or failed.
func (r *sandboxRun) frame(final string) {
	if jsonRequested(r.cmd) {
		return
	}
	elapsed := time.Since(r.started)
	labels := []string{"Creating new sandbox", "Refreshing org", "Authenticating"}
	current := 0
	if r.refresh {
		current = 1
	}
	if r.authing {
		current = 2
	}
	stages := make([]stage, len(labels))
	for i, label := range labels {
		stages[i] = stage{label: label}
		switch {
		case i == 0 && r.refresh:
			stages[i].state = "skipped"
		case i < current:
			stages[i].state = "done"
		case i == current:
			stages[i].state = final
			stages[i].elapsed = elapsed
		}
		if i == 0 && stages[i].state != "" && stages[i].state != "skipped" && r.latest != nil {
			if name := text(r.latest.values["SandboxName"]); name != "" {
				stages[i].details = append(stages[i].details, "Name: "+name)
			}
			if id := text(r.latest.values["Id"]); id != "" {
				stages[i].details = append(stages[i].details, "ID: "+id)
			}
		}
	}
	var info [][2]string
	progress := "0"
	if r.latest != nil {
		if status := text(r.latest.values["Status"]); status != "" {
			info = append(info, [2]string{"Status", paint(r.cmd.OutOrStdout(), styleBold, status)})
		}
		if r.latest.values["CopyProgress"] != nil {
			progress = displayValue(r.latest.values["CopyProgress"])
		}
	}
	info = append(info, [2]string{"Copy progress", progress + "%"}, [2]string{"Elapsed Time", formatElapsed(elapsed)})
	writeStageFrame(r.cmd.OutOrStdout(), r.title, "", false, stages, info)
}

// response is the result of the command: the sandbox process with the
// username of the sandbox, when it is known.
func (r *sandboxRun) response() *orderedMap {
	result := newOrderedMap()
	if r.latest != nil {
		for _, key := range r.latest.keys {
			result.Set(key, r.latest.values[key])
		}
	}
	username := r.username
	if username == "" && r.prod != nil {
		name := ""
		if r.entry != nil {
			if process, ok := r.entry.values["sandboxProcessObject"].(*orderedMap); ok {
				name = text(process.values["SandboxName"])
			}
		}
		if name == "" && r.latest != nil {
			name = text(r.latest.values["SandboxName"])
		}
		if name != "" {
			username = r.prod.Username + "." + name
		}
	}
	if username != "" {
		result.Set("SandboxUsername", username)
	}
	return result
}

// unfinished reports a sandbox that is not ready: it prints the display
// and how to check the request later.
func (r *sandboxRun) unfinished(timedOut bool) {
	r.saveEntry()
	r.frame("async")
	if timedOut {
		warn(r.cmd, fmt.Sprintf("The wait time for the sandbox %s has been exhausted. See the results below for more information.", strings.ToLower(r.action)))
	}
	if jsonRequested(r.cmd) {
		return
	}
	arguments := ""
	if r.latest != nil {
		arguments += " --job-id " + text(r.latest.values["Id"])
	}
	if r.prod != nil {
		arguments += " -o " + r.prod.Username
	}
	w := r.cmd.OutOrStdout()
	fmt.Fprintln(w, paint(w, styleDim, fmt.Sprintf("Run \""+binName+" org resume sandbox%s\" to check for status.\nIf the org is ready, checking the status also authorizes the org for use with Salesforce CLI.", arguments)))
}

// tracking returns the tracksSource value for the sandbox.
func (r *sandboxRun) tracking() bool {
	if r.entry != nil {
		if request, ok := r.entry.values["sandboxRequest"].(*orderedMap); ok {
			if license := text(request.values["LicenseType"]); license == "Partial" || license == "Full" {
				return false
			}
		}
	}
	if r.tracksSource != nil && !*r.tracksSource {
		return false
	}
	if r.prod != nil {
		records, err := toolingRecords(r.prod, "SELECT IsSourceTrackingSandboxesEnabled FROM SourceTrackingSettings")
		if err != nil || len(records) != 1 || !boolValue(records[0].values["IsSourceTrackingSandboxesEnabled"]) {
			return false
		}
	}
	return true
}

// finished records a sandbox that is ready and authorized: it sets the
// alias, the default org and the tracking setting, removes the cache entry
// and prints the outcome.
func (r *sandboxRun) finished(process *orderedMap, username string) error {
	r.latest = process
	r.username = username
	r.saveEntry()
	target := username
	if r.alias != "" {
		if err := lib.SetSFDXAlias(r.alias, username); err != nil {
			return err
		}
		target = r.alias
	}
	if r.setDefault {
		_, inProject := lib.SFLocalConfigPath()
		if _, err := lib.SetSFConfig(lib.SFConfigTargetOrg, target, !inProject); err != nil {
			return err
		}
	}
	if err := lib.UpdateSFDXAuthFields(username, lib.SFDXAuthFields{"tracksSource": r.tracking()}); err != nil {
		return err
	}
	r.frame("done")
	r.removeEntry()
	if !jsonRequested(r.cmd) {
		w := r.cmd.OutOrStdout()
		fmt.Fprintln(w, paint(w, styleGreen, fmt.Sprintf("Your sandbox is ready.\nYou can open it by running \""+binName+" org open -o %s\"", username)))
	}
	return nil
}

// authorize tries to authorize the sandbox of a process.  It returns true
// when the sandbox is authorized, and false when it is not ready.  An error
// of the type sandboxLoginError is a failure to log in to the sandbox or to
// write its files, after the production org granted the authorization.
func (r *sandboxRun) authorize(process *orderedMap) (bool, error) {
	granted, err := sandboxAuthorization(r.prod, process)
	if err != nil {
		return false, err
	}
	if granted == nil {
		return false, nil
	}
	r.username = text(granted.values["authUserName"])
	r.authing = true
	username, err := writeSandboxAuthorization(r.prod, process, granted)
	if err != nil {
		return false, sandboxLoginError{err}
	}
	return true, r.finished(process, username)
}

type sandboxLoginError struct{ cause error }

func (e sandboxLoginError) Error() string { return e.cause.Error() }
func (e sandboxLoginError) Unwrap() error { return e.cause }

// poll checks the sandbox process until the sandbox is authorized or the
// wait is over, in which case it returns errSandboxTimeout.
func (r *sandboxRun) poll(processID string, wait, interval time.Duration) error {
	remaining := wait
	for {
		process, _, err := sandboxProcess(r.prod, fmt.Sprintf("Id='%s'", processID))
		if err != nil {
			return err
		}
		done, err := r.authorize(process)
		if err != nil {
			var login sandboxLoginError
			if errors.As(err, &login) {
				err = login.cause
			}
			var failure *Error
			retry := errors.As(err, &failure) && (failure.Name == "SandboxAuthIncompleteError" ||
				(failure.Name == "JwtAuthError" && strings.Contains(failure.Message, "user hasn't approved")))
			if !retry {
				r.latest = process
				return err
			}
			r.authing = false
		}
		if done {
			return nil
		}
		r.latest = process
		r.saveEntry()
		remaining -= interval
		if remaining < 0 {
			return errSandboxTimeout
		}
		sandboxSleep(interval)
	}
}

// upperFirstKeys returns a JSON object with the first letter of each key in
// upper case: the definition file has the keys of the API in lower camel
// case.
func upperFirstKeys(source *orderedMap) *orderedMap {
	result := newOrderedMap()
	for _, key := range source.keys {
		runes := []rune(key)
		if len(runes) > 0 {
			runes[0] = unicode.ToUpper(runes[0])
		}
		result.Set(string(runes), source.values[key])
	}
	return result
}

// readSandboxDefinition reads a sandbox definition file.
func readSandboxDefinition(path string) (*orderedMap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	parsed, err := parseOrderedJSON(data)
	if err != nil {
		return nil, err
	}
	object, ok := parsed.(*orderedMap)
	if !ok {
		return nil, fmt.Errorf("%s is not a JSON object", path)
	}
	definition := upperFirstKeys(object)
	has := func(key string) bool {
		value, ok := definition.Get(key)
		return ok && value != nil && value != "" && value != false
	}
	switch {
	case has("ApexClassName") && has("ApexClassId"):
		return nil, newError("BothApexClassIdAndNameProvidedError", "You can't specify both `apexClassId` and `apexClassName` in the definition file at the same time.")
	case has("ActivationUserGroupId") && has("ActivationUserGroupName"):
		return nil, newError("BothUserGroupIdAndNameProvidedError", "You can't specify both `ActivationUserGroupId` and `ActivationUserGroupName` in the definition file at the same time.")
	case has("SourceId") && has("SourceSandboxName"):
		return nil, newError("BothSourceIdAndNameProvidedError", "You can't specify both `SourceId` and `SourceSandboxName` in the definition file at the same time.")
	case has("SourceId") && has("LicenseType"):
		return nil, newError("BothSourceIdAndLicenseTypeProvidedError", "You can't specify both `SourceId` and `LicenseType` in the definition file at the same time.")
	case has("LicenseType") && has("SourceSandboxName"):
		return nil, newError("BothSourceSandboxNameAndLicenseTypeProvidedError", "You can't specify both `SourceSandboxName` and `LicenseType` in the definition file at the same time.")
	}
	return definition, nil
}

func sandboxApexClassID(org *targetOrg, name string) (string, error) {
	result, err := runQuery(org, "", "query", fmt.Sprintf("SELECT Id FROM ApexClass WHERE Name = '%s'", name))
	if err == nil {
		if records, _ := result.values["records"].([]interface{}); len(records) == 1 {
			if record, ok := records[0].(*orderedMap); ok {
				return text(record.values["Id"]), nil
			}
		}
	}
	return "", newError("ApexClassQueryFailedError", "Unable to find the ID of the Apex class \"%s\" that's defined in the definition file.", name)
}

func sandboxUserGroupID(org *targetOrg, name string) (string, error) {
	result, err := runQuery(org, "", "query", fmt.Sprintf("SELECT id FROM Group WHERE NAME = '%s'", name))
	if err == nil {
		if records, _ := result.values["records"].([]interface{}); len(records) == 1 {
			if record, ok := records[0].(*orderedMap); ok {
				return text(record.values["Id"]), nil
			}
		}
	}
	return "", newError("UserGroupQueryFailedError", "Unable to find the ID of the activation user group \"%s\" that's defined in the definition file.", name)
}

func sandboxInfoIDByName(org *targetOrg, name string) (string, error) {
	records, err := toolingRecords(org, fmt.Sprintf("SELECT id FROM SandboxInfo WHERE SandboxName = '%s'", name))
	if err != nil || len(records) != 1 {
		return "", newError("SandboxNameQueryFailedError", "Unable to find the ID of the sandbox \"%s\" that's defined in the definition file or as a command line argument.", name)
	}
	return text(records[0].values["Id"]), nil
}

// sandboxFlagConflicts reports flags that sf does not accept together.
// Each pair names two flags; an error line is written for each flag of a
// pair in which both are set.
func sandboxFlagConflicts(cmd *cobra.Command, pairs [][2]string) error {
	var lines []string
	describe := func(name string) string {
		flag := cmd.Flags().Lookup(name)
		if flag.Value.Type() == "bool" {
			return "--" + name
		}
		return "--" + name + "=" + flag.Value.String()
	}
	for _, pair := range pairs {
		if cmd.Flags().Changed(pair[0]) && cmd.Flags().Changed(pair[1]) {
			lines = append(lines,
				fmt.Sprintf("  %s cannot also be provided when using --%s", describe(pair[0]), pair[1]),
				fmt.Sprintf("  %s cannot also be provided when using --%s", describe(pair[1]), pair[0]))
		}
	}
	if len(lines) == 0 {
		return nil
	}
	sort.Strings(lines)
	heading := "The following errors occurred:"
	if len(lines) == 1 {
		heading = "The following error occurred:"
	}
	return &Error{Message: heading + "\n" + strings.Join(lines, "\n") + "\nSee more help with --help", ExitCode: 2}
}

// sandboxWaitFlags reads --wait (minutes) and --poll-interval (seconds).
func sandboxWaitFlags(cmd *cobra.Command, larger string) (time.Duration, time.Duration, error) {
	minutes, _ := cmd.Flags().GetInt("wait")
	seconds, _ := cmd.Flags().GetInt("poll-interval")
	if minutes < 1 {
		return 0, 0, flagParseError("DurationBoundsMinError", "wait", "The value must be at least 1.")
	}
	if seconds < 15 {
		return 0, 0, flagParseError("DurationBoundsMinError", "poll-interval", "The value must be at least 15.")
	}
	if seconds > minutes*60 {
		return 0, 0, newError("PollIntervalGreaterThanWaitError", larger, seconds, minutes*60)
	}
	return time.Duration(minutes) * time.Minute, time.Duration(seconds) * time.Second, nil
}

// sandboxDefinitionConflicts checks the clone flags against the clone
// options of the definition file.
func sandboxDefinitionConflicts(cmd *cobra.Command, definition *orderedMap, verb string) error {
	if definition == nil {
		return nil
	}
	sourceID, _ := cmd.Flags().GetString("source-id")
	sourceName, _ := cmd.Flags().GetString("source-sandbox-name")
	fileID, fileName := text(definition.values["SourceId"]), text(definition.values["SourceSandboxName"])
	switch {
	case sourceID != "" && fileID != "":
		return newError("BothIdFlagAndDefFilePropertyAreProvidedError", "You can't specify both the --source-id and --definition-file flags, and also include the \"SourceId\" option in the definition file. Pick one method of %s a sandbox and try again.", verb)
	case sourceName != "" && fileName != "":
		return newError("BothNameFlagAndDefFilePropertyAreProvidedError", "You can't specify both the --source-sandbox-name and --definition-file flags, and also include the \"SourceSandboxName\" option in the definition file. Pick one method of %s a sandbox and try again.", verb)
	case (sourceID != "" && fileName != "") || (sourceName != "" && fileID != ""):
		return newError("BothIdFlagAndNameDefFileAreNotAllowedError", "You can't specify both the --source-sandbox-name and --definition-file flags, and also include the \"SourceId\" option in the definition file. Same with the --source-id flag and \"SourceSandboxName\" option. Pick one method of %s a sandbox and try again.", verb)
	}
	return nil
}

// confirmSandboxRequest shows the request and asks whether it is correct.
func confirmSandboxRequest(cmd *cobra.Command, title string, request *orderedMap, nullText bool) error {
	if noPrompt, _ := cmd.Flags().GetBool("no-prompt"); noPrompt || jsonRequested(cmd) {
		return nil
	}
	rows := make([][]string, 0, len(request.keys))
	for _, key := range request.keys {
		value := ""
		switch {
		case request.values[key] != nil:
			value = displayValue(request.values[key])
		case nullText:
			value = "null"
		}
		rows = append(rows, []string{key, value})
	}
	printTable(cmd.OutOrStdout(), title, []string{"Field", "Value"}, rows)
	if !confirm(cmd, "Is the configuration correct?") {
		return newError("UserNotSatisfiedWithSandboxConfigError", "The sandbox request configuration isn't acceptable.")
	}
	return nil
}

// jsonText returns a list value as the JSON text the Tooling API expects
// for the Features and PostCopyConfig fields.
func jsonText(value interface{}) interface{} {
	if list, ok := value.([]interface{}); ok {
		if encoded, err := json.Marshal(list); err == nil {
			return string(encoded)
		}
	}
	return value
}

func sandboxNameFlag(cmd *cobra.Command, message string) (string, error) {
	name, _ := cmd.Flags().GetString("name")
	if len(name) > 10 {
		return "", flagParseError("SandboxNameLengthError", "name", fmt.Sprintf(message, name))
	}
	return name, nil
}

func newOrgCreateSandboxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "sandbox",
		Short:       "Create a sandbox org.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "CreateSandbox"},
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := sandboxNameFlag(cmd, "The sandbox name \"%s\" should be 10 or fewer characters.")
			if err != nil {
				return err
			}
			if err := sandboxFlagConflicts(cmd, [][2]string{
				{"async", "wait"}, {"async", "poll-interval"},
				{"source-sandbox-name", "license-type"}, {"source-sandbox-name", "source-id"}, {"source-id", "license-type"},
			}); err != nil {
				return err
			}
			license, _ := cmd.Flags().GetString("license-type")
			if license != "" {
				valid := false
				for _, option := range sandboxLicenseTypes {
					valid = valid || option == license
				}
				if !valid {
					return &Error{Message: fmt.Sprintf("Expected --license-type=%s to be one of: %s\nSee more help with --help", license, strings.Join(sandboxLicenseTypes, ", ")), ExitCode: 2}
				}
			}
			sourceID, _ := cmd.Flags().GetString("source-id")
			if sourceID != "" {
				if err := salesforceIDFlag("source-id", sourceID, "0GQ", true); err != nil {
					return err
				}
			}
			sourceName, _ := cmd.Flags().GetString("source-sandbox-name")
			definitionFile, _ := cmd.Flags().GetString("definition-file")
			if definitionFile != "" {
				if _, err := os.Stat(definitionFile); err != nil {
					return flagParseError("", "definition-file", "No file found at "+definitionFile)
				}
			}
			wait, interval, err := sandboxWaitFlags(cmd, "The poll interval (%d seconds) can't be larger that wait (%d in seconds)")
			if err != nil {
				return err
			}
			definition := newOrderedMap()
			if definitionFile != "" {
				if definition, err = readSandboxDefinition(definitionFile); err != nil {
					return err
				}
				if err := sandboxDefinitionConflicts(cmd, definition, "cloning"); err != nil {
					return err
				}
			}
			prod, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}

			// Flags override the definition file.
			merged := newOrderedMap()
			for _, key := range definition.keys {
				merged.Set(key, definition.values[key])
			}
			if name != "" {
				merged.Set("SandboxName", name)
			}
			switch {
			case sourceName != "":
				merged.Set("SourceSandboxName", sourceName)
			case sourceID != "":
				merged.Set("SourceId", sourceID)
			case license != "":
				merged.Set("LicenseType", license)
			}
			if text(merged.values["SandboxName"]) == "" {
				stamp := strconv.FormatInt(time.Now().UnixMilli(), 36)
				generated := "sbx" + stamp[len(stamp)-7:]
				warn(cmd, "No SandboxName defined, generating new SandboxName: "+generated)
				merged.Set("SandboxName", generated)
			}
			cloneName, cloneID := text(merged.values["SourceSandboxName"]), text(merged.values["SourceId"])
			merged.Delete("SourceSandboxName")
			merged.Delete("SourceId")
			if cloneName == "" && cloneID == "" && text(merged.values["LicenseType"]) == "" {
				merged.Set("LicenseType", "Developer")
			}
			apexID, groupID := "", ""
			if className := text(merged.values["ApexClassName"]); className != "" {
				if apexID, err = sandboxApexClassID(prod, className); err != nil {
					return err
				}
				merged.Delete("ApexClassName")
			}
			if groupName := text(merged.values["ActivationUserGroupName"]); groupName != "" {
				if groupID, err = sandboxUserGroupID(prod, groupName); err != nil {
					return err
				}
				merged.Delete("ActivationUserGroupName")
			}
			if cloneName != "" || cloneID != "" {
				// A clone has the features of its source.
				where := fmt.Sprintf("SandboxName='%s'", cloneName)
				if cloneID != "" && cloneName == "" {
					where = fmt.Sprintf("Id='%s'", cloneID)
				}
				infos, err := toolingRecords(prod, fmt.Sprintf("SELECT %s FROM SandboxInfo WHERE %s ORDER BY CreatedDate DESC", sandboxInfoFields, where))
				if err != nil {
					return err
				}
				var current []*orderedMap
				for _, info := range infos {
					if !boolValue(info.values["IsDeleted"]) {
						current = append(current, info)
					}
				}
				if len(current) != 1 {
					if len(current) == 0 {
						return newError("SingleRecordQuery_NoRecords", "No record found for SELECT %s FROM SandboxInfo WHERE %s ORDER BY CreatedDate DESC", sandboxInfoFields, where)
					}
					return newError("SingleRecordQuery_MultipleRecords", "The query returned more than 1 record")
				}
				features := current[0].values["Features"]
				if features == nil || features == "" {
					processWhere := fmt.Sprintf("SandboxName='%s'", cloneName)
					if cloneID != "" && cloneName == "" {
						processWhere = fmt.Sprintf("SandboxInfoId='%s'", cloneID)
					}
					process, _, err := sandboxProcess(prod, processWhere)
					if err != nil {
						return err
					}
					features = process.values["Features"]
				}
				if features != nil && features != "" {
					merged.Set("Features", features)
				}
			}
			if cloneName != "" {
				id, err := sandboxInfoIDByName(prod, cloneName)
				if err != nil {
					return err
				}
				merged.Set("SourceId", id)
			}
			if cloneID != "" {
				merged.Set("SourceId", cloneID)
			}
			if apexID != "" {
				merged.Set("ApexClassId", apexID)
			}
			if groupID != "" {
				merged.Set("ActivationUserGroupId", groupID)
			}
			if err := confirmSandboxRequest(cmd, "Config Sandbox Request", merged, false); err != nil {
				return err
			}

			async, _ := cmd.Flags().GetBool("async")
			run := &sandboxRun{cmd: cmd, prod: prod, action: "Create", title: "Sandbox Create", started: time.Now()}
			if async {
				run.title += " (async)"
			}
			run.alias, _ = cmd.Flags().GetString("alias")
			run.setDefault, _ = cmd.Flags().GetBool("set-default")
			if noTracking, _ := cmd.Flags().GetBool("no-track-source"); noTracking {
				off := false
				run.tracksSource = &off
			}
			run.entry = newOrderedMap()
			if run.alias != "" {
				run.entry.Set("alias", run.alias)
			}
			run.entry.Set("setDefault", run.setDefault)
			run.entry.Set("prodOrgUsername", prod.Username)
			run.entry.Set("action", "Create")
			run.entry.Set("sandboxProcessObject", orderedFields("SandboxName", merged.values["SandboxName"]))
			run.entry.Set("sandboxRequest", merged)
			if run.tracksSource != nil {
				run.entry.Set("tracksSource", false)
			}
			run.saveEntry()

			request := newOrderedMap()
			for _, key := range merged.keys {
				value := merged.values[key]
				if key == "Features" || key == "PostCopyConfig" {
					value = jsonText(value)
				}
				request.Set(key, value)
			}
			body, err := json.Marshal(request)
			if err != nil {
				return err
			}
			response, err := prod.Force.PostREST("/tooling/sobjects/SandboxInfo", string(body))
			if err != nil {
				run.frame("failed")
				return apiError(err)
			}
			var created struct {
				ID      string `json:"id"`
				Success bool   `json:"success"`
			}
			if json.Unmarshal([]byte(response), &created) != nil || !created.Success {
				run.frame("failed")
				return newError("SandboxInfoCreateFailedError", "The sandbox org creation failed with a result of %s.", response)
			}
			process, _, err := sandboxProcess(prod, fmt.Sprintf("SandboxInfoId='%s'", created.ID))
			if err != nil {
				run.frame("failed")
				return err
			}
			run.latest = process
			return finishSandboxRequest(run, async, wait, interval)
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().StringP("definition-file", "f", "", "Path to a sandbox definition file.")
	cmd.Flags().BoolP("set-default", "s", false, "Set the sandbox org as your default org.")
	cmd.Flags().StringP("alias", "a", "", "Alias for the sandbox org.")
	cmd.Flags().IntP("wait", "w", 30, "Number of minutes to wait for the sandbox org to be ready.")
	cmd.Flags().IntP("poll-interval", "i", 30, "Number of seconds to wait between retries.")
	cmd.Flags().Bool("async", false, "Request the sandbox creation, but don't wait for it to complete.")
	cmd.Flags().StringP("name", "n", "", "Name of the sandbox org.")
	cmd.Flags().String("source-sandbox-name", "", "Name of the sandbox org to clone.")
	cmd.Flags().String("source-id", "", "ID of the sandbox org to clone.")
	cmd.Flags().StringP("license-type", "l", "", "Type of sandbox license.")
	cmd.Flags().Bool("no-prompt", false, "Don't prompt for confirmation about the sandbox configuration.")
	cmd.Flags().Bool("no-track-source", false, "Do not use source tracking for this sandbox.")
	addTargetOrgFlag(cmd)
	return cmd
}

// finishSandboxRequest reports a sandbox request that was sent: with
// --async it returns the sandbox process, and otherwise it waits for the
// sandbox.  A sandbox that is not ready gives the exit status 68.
func finishSandboxRequest(run *sandboxRun, async bool, wait, interval time.Duration) error {
	cmd := run.cmd
	if async {
		run.unfinished(false)
		if err := emit(cmd, run.response(), nil); err != nil {
			return err
		}
		return exitStatus(68)
	}
	err := run.poll(text(run.latest.values["Id"]), wait, interval)
	switch {
	case err == nil:
		return emit(cmd, run.response(), nil)
	case errors.Is(err, errSandboxTimeout):
		run.unfinished(true)
		if err := emitStatus(cmd, 68, run.response(), nil); err != nil {
			return err
		}
		return exitStatus(68)
	}
	run.frame("failed")
	return err
}

func newOrgRefreshSandboxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "sandbox",
		Short:       "Refresh a sandbox org using the sandbox name.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "RefreshSandbox"},
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := sandboxNameFlag(cmd, "The sandbox name \"%s\" must be 10 or fewer characters.")
			if err != nil {
				return err
			}
			if err := sandboxFlagConflicts(cmd, [][2]string{{"async", "wait"}, {"async", "poll-interval"}, {"source-sandbox-name", "source-id"}}); err != nil {
				return err
			}
			sourceID, _ := cmd.Flags().GetString("source-id")
			if sourceID != "" && len(sourceID) != 15 && len(sourceID) != 18 {
				return flagParseError("InvalidIdLengthError", "source-id", "The id must be 15 or 18 characters.")
			}
			sourceName, _ := cmd.Flags().GetString("source-sandbox-name")
			definitionFile, _ := cmd.Flags().GetString("definition-file")
			if definitionFile != "" {
				if _, err := os.Stat(definitionFile); err != nil {
					return flagParseError("", "definition-file", "No file found at "+definitionFile)
				}
			}
			wait, interval, err := sandboxWaitFlags(cmd, "The poll interval (%d seconds) can't be larger than the wait period (%d in seconds).")
			if err != nil {
				return err
			}
			definition := newOrderedMap()
			if definitionFile != "" {
				if definition, err = readSandboxDefinition(definitionFile); err != nil {
					return err
				}
				if err := sandboxDefinitionConflicts(cmd, definition, "refreshing"); err != nil {
					return err
				}
			}
			prod, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			fileName := text(definition.values["SandboxName"])
			if fileName == "" && name == "" {
				return newError("NoSandboxNameError", "Must specify a sandbox name using the `--name` or `--definition-file` flag.")
			}
			apexID, groupID, fileSourceID := "", "", ""
			if className := text(definition.values["ApexClassName"]); className != "" {
				if apexID, err = sandboxApexClassID(prod, className); err != nil {
					return err
				}
				definition.Delete("ApexClassName")
			}
			if groupName := text(definition.values["ActivationUserGroupName"]); groupName != "" {
				if groupID, err = sandboxUserGroupID(prod, groupName); err != nil {
					return err
				}
				definition.Delete("ActivationUserGroupName")
			}
			if source := text(definition.values["SourceSandboxName"]); source != "" {
				if fileSourceID, err = sandboxInfoIDByName(prod, source); err != nil {
					return err
				}
				definition.Delete("SourceSandboxName")
			}
			if fileName != "" && name != "" && name != fileName {
				warn(cmd, fmt.Sprintf("Different sandbox names were provided with the `--name` ('%s') and `--definition-file` flags ('%s'). Using the value provided by the `--name` flag. If you want to change the name of the sandbox, first delete it and then create it again using the new name.", name, fileName))
			}
			if name == "" {
				name = fileName
			}
			records, err := toolingRecords(prod, fmt.Sprintf("SELECT Id,SandboxName,LicenseType,HistoryDays,CopyChatter,AutoActivate,Description,SourceId,IsDeleted,CreatedDate,CreatedById,LastModifiedDate,LastModifiedById FROM SandboxInfo WHERE SandboxName='%s'", name))
			if err != nil {
				return err
			}
			if len(records) == 0 {
				return &Error{
					Name:     "SandboxNotFoundError",
					Message:  fmt.Sprintf("The sandbox name \"%s\" could not be found in production org \"%s\".", name, prod.Username),
					ExitCode: 1,
					Actions:  []string{"Ensure the sandbox name and casing is correct.", "Ensure the production org for the sandbox is correct."},
				}
			}
			if len(records) > 1 {
				return newError("SingleRecordQuery_MultipleRecords", "The query returned more than 1 record")
			}
			info := newOrderedMap()
			for _, key := range records[0].keys {
				if key != "attributes" {
					info.Set(key, records[0].values[key])
				}
			}
			for _, key := range definition.keys {
				info.Set(key, definition.values[key])
			}
			info.Set("SandboxName", name)
			noActivate, _ := cmd.Flags().GetBool("no-auto-activate")
			info.Set("AutoActivate", !noActivate)
			switch {
			case sourceName != "":
				id, err := sandboxInfoIDByName(prod, sourceName)
				if err != nil {
					return err
				}
				info.Set("SourceId", id)
			case sourceID != "":
				info.Set("SourceId", sourceID)
			}
			if apexID != "" {
				info.Set("ApexClassId", apexID)
			}
			if groupID != "" {
				info.Set("ActivationUserGroupId", groupID)
			}
			if fileSourceID != "" {
				info.Set("SourceId", fileSourceID)
			}
			if err := confirmSandboxRequest(cmd, "Config Sandbox Refresh", info, true); err != nil {
				return err
			}

			async, _ := cmd.Flags().GetBool("async")
			run := &sandboxRun{cmd: cmd, prod: prod, action: "Refresh", title: "Sandbox Refresh", refresh: true, started: time.Now()}
			if async {
				run.title += " (async)"
			}
			run.entry = orderedFields("prodOrgUsername", prod.Username, "action", "Refresh",
				"sandboxProcessObject", orderedFields("SandboxName", name), "sandboxRequest", info)
			run.saveEntry()

			// The fields that cannot be changed are not sent.
			update := newOrderedMap()
			for _, key := range info.keys {
				switch key {
				case "Id", "IsDeleted", "CreatedDate", "CreatedById", "LastModifiedDate", "LastModifiedById":
					continue
				}
				value := info.values[key]
				if key == "PostCopyConfig" {
					value = jsonText(value)
				}
				update.Set(key, value)
			}
			body, err := json.Marshal(update)
			if err != nil {
				return err
			}
			if _, err := prod.Force.PatchREST("/tooling/sobjects/SandboxInfo/"+text(info.values["Id"]), string(body)); err != nil {
				run.frame("failed")
				return apiError(err)
			}
			processes, _, err := sandboxProcesses(prod, fmt.Sprintf("SandboxName='%s'", name))
			if err != nil {
				run.frame("failed")
				return err
			}
			if len(processes) == 0 {
				run.frame("failed")
				return &Error{Message: "No SandboxProcesses found for: " + name, ExitCode: 1}
			}
			run.latest = processes[0]
			return finishSandboxRequest(run, async, wait, interval)
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().Bool("no-auto-activate", false, "Disable auto-activation of the sandbox after a successful refresh.")
	cmd.Flags().IntP("wait", "w", 30, "Number of minutes to poll for sandbox refresh status.")
	cmd.Flags().IntP("poll-interval", "i", 30, "Number of seconds to wait between status polling requests.")
	cmd.Flags().String("source-sandbox-name", "", "Name of the sandbox org that becomes the new source org for the refreshed sandbox.")
	cmd.Flags().String("source-id", "", "ID of the sandbox org that becomes the new source org for the refreshed sandbox.")
	cmd.Flags().Bool("async", false, "Request the sandbox refresh, but don't wait for it to complete.")
	cmd.Flags().StringP("name", "n", "", "Name of the existing sandbox org in your production org that you want to refresh.")
	cmd.Flags().StringP("definition-file", "f", "", "Path to a sandbox definition file for overriding its configuration when you refresh it.")
	cmd.Flags().Bool("no-prompt", false, "Don't prompt for confirmation about the sandbox refresh.")
	addTargetOrgFlag(cmd)
	return cmd
}

var resumableSandboxStatuses = map[string]bool{"Activating": true, "Pending": true, "Pending Activation": true, "Processing": true, "Sampling": true, "Completed": true}

func newOrgResumeSandboxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "sandbox",
		Short:       "Check the status of a sandbox creation, and log in to it if it's ready.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "ResumeSandbox"},
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := sandboxNameFlag(cmd, "The sandbox name \"%s\" should be 10 or fewer characters.")
			if err != nil {
				return err
			}
			if err := sandboxFlagConflicts(cmd, [][2]string{{"job-id", "name"}}); err != nil {
				return err
			}
			jobID, _ := cmd.Flags().GetString("job-id")
			if jobID != "" {
				if err := salesforceIDFlag("job-id", jobID, "0GR", false); err != nil {
					return err
				}
			}
			minutes, _ := cmd.Flags().GetInt("wait")
			if minutes < 0 {
				return flagParseError("DurationBoundsMinError", "wait", "The value must be at least 0.")
			}
			useMostRecent, _ := cmd.Flags().GetBool("use-most-recent")
			cache, err := readSandboxCache()
			if err != nil {
				return err
			}
			var entry *orderedMap
			switch {
			case useMostRecent:
				latestName, latest := latestSandboxCacheEntry(cache)
				if latestName == "" {
					return newError("LatestSandboxRequestNotFoundError", "Retry the command using either the --name or --job-id flags.")
				}
				entry = latest
			case name != "":
				entry, _ = cache.values[name].(*orderedMap)
			case jobID != "":
				for _, key := range cache.keys {
					candidate, ok := cache.values[key].(*orderedMap)
					if !ok {
						continue
					}
					if process, ok := candidate.values["sandboxProcessObject"].(*orderedMap); ok && text(process.values["Id"]) == jobID {
						entry = candidate
						break
					}
				}
				if entry == nil {
					warn(cmd, fmt.Sprintf("Could not find a cache entry for %s.\nIf you are resuming a sandbox operation from a different machine note that we cannot set the alias/set-default flag values as those are saved locally.", jobID))
				}
			}
			run := &sandboxRun{cmd: cmd, action: "Create", title: "Resume Sandbox", started: time.Now()}
			prodUsername := ""
			if entry != nil {
				run.entry = entry
				prodUsername = text(entry.values["prodOrgUsername"])
				if action := text(entry.values["action"]); action != "" {
					run.action = action
				}
				run.alias = text(entry.values["alias"])
				run.setDefault = boolValue(entry.values["setDefault"])
				if value, ok := entry.Get("tracksSource"); ok && value != nil {
					tracks := boolValue(value)
					run.tracksSource = &tracks
				}
			} else {
				run.entry = orderedFields("sandboxProcessObject", orderedFields("SandboxName", name), "sandboxRequest", newOrderedMap(), "setDefault", false)
			}
			run.refresh = run.action == "Refresh"
			cachedProcess, _ := run.entry.values["sandboxProcessObject"].(*orderedMap)
			cachedName := ""
			if cachedProcess != nil {
				cachedName = text(cachedProcess.values["SandboxName"])
			}
			if cachedName == "" && name == "" && jobID == "" {
				return newError("NoSandboxNameOrJobIdError", "No sandbox name or job ID were provided.")
			}
			if prodUsername != "" {
				if run.prod, err = loadOrg(prodUsername); err != nil {
					return newError("NamedOrgNotFoundError", "No authorization information found for %s.", prodUsername)
				}
			} else if run.prod, err = resolveTargetOrg(cmd); err != nil {
				return err
			}
			run.entry.Set("prodOrgUsername", run.prod.Username)
			run.entry.Set("action", run.action)

			// The sandbox process to resume.
			var process *orderedMap
			processID := jobID
			if useMostRecent && cachedProcess != nil && text(cachedProcess.values["Id"]) != "" {
				processID = text(cachedProcess.values["Id"])
			}
			switch {
			case processID != "":
				process, _, err = sandboxProcess(run.prod, fmt.Sprintf("Id='%s'", processID))
			case name != "":
				var several []*orderedMap
				process, several, err = sandboxProcess(run.prod, fmt.Sprintf("SandboxName='%s'", name))
				if len(several) > 0 {
					// The newest process is resumed.
					process, err = several[0], nil
					var ids, statuses []string
					for _, other := range several[1:] {
						ids = append(ids, text(other.values["Id"]))
						statuses = append(statuses, text(other.values["Status"]))
					}
					warn(cmd, fmt.Sprintf("We found multiple sandbox processes for \"%s\" in a resumable state. We're ignoring the sandbox process ID(s) \"%s\" in status(es) \"%s\" and using the most recent process ID \"%s\". To resume a different sandbox process, use that unique sandbox process ID with the command. For example, \""+binName+" org resume sandbox --job-id %s -o %s\".",
						text(several[1].values["SandboxName"]), strings.Join(ids, ","), strings.Join(statuses, ","), text(process.values["Id"]), ids[0], run.prod.Username))
				}
			default:
				return newError("SandboxNotFoundError", "We can't find a SandboxProcess for the sandbox %s.", name)
			}
			if err != nil {
				run.frame("failed")
				return err
			}
			if status := text(process.values["Status"]); !resumableSandboxStatuses[status] {
				run.frame("failed")
				return newError("SandboxNotResumableError", "The sandbox %s cannot resume with status of %s.", text(process.values["SandboxName"]), status)
			}
			run.latest = process
			run.saveEntry()

			incomplete := func(message string) error {
				result := run.response()
				result.Set("Message", message)
				if err := emitStatus(cmd, 68, result, nil); err != nil {
					return err
				}
				return exitStatus(68)
			}
			wait := time.Duration(minutes) * time.Minute
			if wait == 0 {
				notComplete := "The sandbox creation has not completed."
				if text(process.values["Status"]) == "Completed" {
					done, err := run.authorize(process)
					if err != nil {
						run.frame("failed")
						// A failure to log in to the sandbox or to write
						// its files is reported in the result, with the
						// exit status 68.  So is an authorization that the
						// production org is not ready to give.  Any other
						// error of the authorization request is an error
						// of the command.
						var login sandboxLoginError
						var failure *Error
						switch {
						case errors.As(err, &login):
							message := login.cause.Error()
							if errors.As(login.cause, &failure) {
								message = failure.Message
							}
							return incomplete(fmt.Sprintf("The sandbox org completed successfully, but the local authentication file could not be written: %s. Run \"org resume sandbox\" again to retry authentication.", message))
						case errors.As(err, &failure) && failure.Name == "SandboxAuthIncompleteError":
							return incomplete(failure.Message)
						}
						return err
					}
					if done {
						return emit(cmd, run.response(), nil)
					}
				}
				run.unfinished(false)
				return incomplete(notComplete)
			}
			err = run.poll(text(process.values["Id"]), wait, 30*time.Second)
			switch {
			case err == nil:
				return emit(cmd, run.response(), nil)
			case errors.Is(err, errSandboxTimeout):
				run.unfinished(true)
				return incomplete(err.Error())
			}
			run.frame("failed")
			return err
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().IntP("wait", "w", 0, "Number of minutes to wait for the sandbox org to be ready.")
	cmd.Flags().StringP("name", "n", "", "Name of the sandbox org.")
	cmd.Flags().StringP("job-id", "i", "", "Job ID of the incomplete sandbox creation that you want to check the status of.")
	cmd.Flags().BoolP("use-most-recent", "l", false, "Use the most recent sandbox create request.")
	addTargetOrgFlag(cmd)
	return cmd
}
