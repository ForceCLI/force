package sf

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ForceCLI/force/lib"
)

func newOrgDeleteScratchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "scratch",
		Short:       "Delete a scratch org.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "DeleteScratch"},
		RunE: func(cmd *cobra.Command, args []string) error {
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				if e, ok := err.(*Error); ok && e.Name == "NoDefaultEnvError" {
					return newError("MissingUsernameError", "Unable to determine the username of the org to delete. Specify the username with the --target-org | -o flag.")
				}
				return err
			}
			devHub := org.Fields.String("devHubUsername")
			if !org.Fields.Bool("isScratch") && devHub == "" {
				return newError("UnknownScratchError", "Unable to find a scratch org with username %s.", org.Username)
			}
			result := newOrderedMap()
			result.Set("username", org.Username)
			result.Set("orgId", org.Fields.String("orgId"))
			if !confirm(cmd, fmt.Sprintf("Are you sure you want to delete the scratch org with name: %s?", org.Username)) {
				return emit(cmd, result, nil)
			}
			hub, err := loadOrg(devHub)
			if err != nil {
				return newError("NamedOrgNotFoundError", "No authorization information found for %s.", devHub)
			}
			soql := fmt.Sprintf("SELECT Id FROM ActiveScratchOrg WHERE SignupUsername='%s'", strings.ReplaceAll(org.Username, "'", `\'`))
			active, err := runQuery(hub, "", "query", soql)
			if err != nil {
				return apiError(err)
			}
			records, _ := active.values["records"].([]interface{})
			// sf prints nothing after it deleted the org, and a message
			// when the Dev Hub no longer has the org.
			message := fmt.Sprintf("Successfully deleted scratch org %s.", org.Username)
			if len(records) > 0 {
				record, _ := records[0].(*orderedMap)
				if _, err := hub.Force.DeleteREST("/sobjects/ActiveScratchOrg/" + text(record.values["Id"])); err != nil {
					return apiError(err)
				}
				message = ""
			}
			if err := logout(org.Username); err != nil {
				return err
			}
			return emit(cmd, result, func(w io.Writer) {
				if message != "" {
					fmt.Fprintln(w, paint(w, styleGreen, message))
				}
			})
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().BoolP("no-prompt", "p", false, "Don't prompt the user to confirm the deletion.")
	addTargetOrgFlag(cmd)
	return cmd
}

func newOrgListSobjectRecordCountsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "record-counts",
		Short:       "Display record counts for the specified standard or custom objects.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "LimitsRecordCountsDisplayCommand"},
		RunE: func(cmd *cobra.Command, args []string) error {
			sobjects := splitList(cmd, "sobject")
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			resource := "/limits/recordCount?sObjects"
			if len(sobjects) > 0 {
				resource += "=" + url.QueryEscape(strings.Join(sobjects, ","))
			}
			body, err := org.Force.GetREST(resource)
			if err != nil {
				converted := apiError(err)
				if e, ok := converted.(*Error); ok {
					e.Label, e.Data = "", nil
				}
				return converted
			}
			parsed, err := parseOrderedJSON([]byte(body))
			if err != nil {
				return err
			}
			counts := []*orderedMap{}
			if response, ok := parsed.(*orderedMap); ok {
				items, _ := response.values["sObjects"].([]interface{})
				for _, item := range items {
					if record, ok := item.(*orderedMap); ok && (len(sobjects) == 0 || containsString(sobjects, text(record.values["name"]))) {
						counts = append(counts, record)
					}
				}
			}
			sort.SliceStable(counts, func(i, j int) bool { return text(counts[i].values["name"]) < text(counts[j].values["name"]) })
			rows := make([][]string, 0, len(counts))
			for _, record := range counts {
				rows = append(rows, []string{text(record.values["name"]), displayValue(record.values["count"])})
			}
			return emit(cmd, counts, func(w io.Writer) {
				printTable(w, "", []string{"sObject", "Record Count"}, rows)
			})
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().StringArrayP("sobject", "s", nil, "API name of an sObject that you want to display the total count for.")
	addAPIVersionFlag(cmd)
	addTargetOrgFlag(cmd)
	return cmd
}

// userID returns the Id of the user with a username or alias in the org.
func userID(org *targetOrg, aliasOrUsername string) (string, error) {
	username := lib.ResolveSFDXUsername(aliasOrUsername)
	soql := fmt.Sprintf("SELECT Id FROM User WHERE Username='%s'", strings.ReplaceAll(username, "'", `\'`))
	result, err := runQuery(org, "", "query", soql)
	if err != nil {
		return "", apiError(err)
	}
	records, _ := result.values["records"].([]interface{})
	if len(records) == 0 {
		return "", newError("SingleRecordQuery_NoRecords", "No record found for %s", soql)
	}
	record, _ := records[0].(*orderedMap)
	return text(record.values["Id"]), nil
}

type assignment struct {
	name    string
	value   string
	message string
}

// emitAssignments prints the result of the org assign commands.  The exit
// status is 1 when every assignment failed and 68 when some failed.
func emitAssignments(cmd *cobra.Command, successes, failures []assignment, successTitle, valueColumn string) error {
	successItems := []*orderedMap{}
	failureItems := []*orderedMap{}
	var successRows, failureRows [][]string
	for _, success := range successes {
		item := newOrderedMap()
		item.Set("name", success.name)
		item.Set("value", success.value)
		successItems = append(successItems, item)
		successRows = append(successRows, []string{success.name, success.value})
	}
	for _, failure := range failures {
		item := newOrderedMap()
		item.Set("name", failure.name)
		item.Set("message", failure.message)
		failureItems = append(failureItems, item)
		failureRows = append(failureRows, []string{failure.name, failure.message})
	}
	status := 0
	switch {
	case len(failures) > 0 && len(successes) > 0:
		status = 68
	case len(failures) > 0:
		status = 1
	}
	result := newOrderedMap()
	result.Set("successes", successItems)
	result.Set("failures", failureItems)
	if err := emitStatus(cmd, status, result, func(w io.Writer) {
		if len(successRows) > 0 {
			title := successTitle
			if len(failureRows) > 0 {
				title = "Partial Success"
			}
			printTable(w, title, []string{"Username", valueColumn}, successRows)
			if len(failureRows) > 0 {
				fmt.Fprintln(w)
			}
		}
		if len(failureRows) > 0 {
			printTable(w, "Failures", []string{"Username", "Error Message"}, failureRows)
		}
	}); err != nil {
		return err
	}
	if status != 0 {
		return exitStatus(status)
	}
	return nil
}

func addAssignFlags(cmd *cobra.Command, nameUsage string) {
	addJSONFlag(cmd)
	cmd.Flags().StringArrayP("name", "n", nil, nameUsage)
	cmd.Flags().StringArrayP("on-behalf-of", "b", nil, "Username or alias to assign to.")
	addAPIVersionFlag(cmd)
	addTargetOrgFlag(cmd)
}

// assignTo runs assign for each user and each name and collects the
// outcomes.
func assignTo(cmd *cobra.Command, org *targetOrg, find func(name string) (string, error), create func(userID, id string) error) (successes, failures []assignment, err error) {
	names := splitList(cmd, "name")
	if len(names) == 0 {
		return nil, nil, missingFlagError("name")
	}
	users, _ := cmd.Flags().GetStringArray("on-behalf-of")
	if len(users) == 0 {
		users = []string{org.Username}
	}
	for _, user := range users {
		id, err := userID(org, user)
		if err != nil {
			return nil, nil, err
		}
		for _, name := range names {
			targetID, err := find(name)
			if err == nil {
				err = create(id, targetID)
			}
			if err != nil {
				message := err.Error()
				if e, ok := apiError(err).(*Error); ok {
					message = e.Message
				}
				failures = append(failures, assignment{name: user, message: message})
				continue
			}
			successes = append(successes, assignment{name: user, value: name})
		}
	}
	return successes, failures, nil
}

func firstRecordID(org *targetOrg, soql string) (string, bool, error) {
	result, err := runQuery(org, "", "query", soql)
	if err != nil {
		return "", false, err
	}
	records, _ := result.values["records"].([]interface{})
	if len(records) == 0 {
		return "", false, nil
	}
	record, _ := records[0].(*orderedMap)
	return text(record.values["Id"]), true, nil
}

func createRecord(org *targetOrg, sobject string, fields *orderedMap) error {
	content, err := encodeJSON(fields)
	if err != nil {
		return err
	}
	_, err = org.Force.PostREST("/sobjects/"+sobject, content)
	return err
}

func newOrgAssignPermsetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "permset",
		Short:       "Assign a permission set to one or more org users.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "AssignPermSetCommand"},
		RunE: func(cmd *cobra.Command, args []string) error {
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			successes, failures, err := assignTo(cmd, org,
				func(name string) (string, error) { return permissionSetID(org, name) },
				func(userID, permissionSetID string) error {
					fields := newOrderedMap()
					fields.Set("AssigneeId", userID)
					fields.Set("PermissionSetId", permissionSetID)
					return createRecord(org, "PermissionSetAssignment", fields)
				})
			if err != nil {
				return err
			}
			return emitAssignments(cmd, successes, failures, "Permsets Assigned", "Permission Set Assignment")
		},
	}
	addAssignFlags(cmd, "Permission set to assign.")
	return cmd
}

func newOrgAssignPermsetLicenseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "permsetlicense",
		Short:       "Assign a permission set license to one or more org users.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "AssignPermSetLicenseCommand"},
		RunE: func(cmd *cobra.Command, args []string) error {
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			successes, failures, err := assignTo(cmd, org,
				func(name string) (string, error) {
					escaped := strings.ReplaceAll(name, "'", `\'`)
					id, found, err := firstRecordID(org, fmt.Sprintf("SELECT Id FROM PermissionSetLicense WHERE DeveloperName='%s' OR MasterLabel='%s'", escaped, escaped))
					if err != nil {
						return "", err
					}
					if !found {
						return "", fmt.Errorf("PermissionSetLicense not found: %s", name)
					}
					return id, nil
				},
				func(userID, licenseID string) error {
					fields := newOrderedMap()
					fields.Set("AssigneeId", userID)
					fields.Set("PermissionSetLicenseId", licenseID)
					return createRecord(org, "PermissionSetLicenseAssign", fields)
				})
			if err != nil {
				return err
			}
			return emitAssignments(cmd, successes, failures, "Permset Licenses Assigned", "Permission Set License Assignment")
		},
	}
	addAssignFlags(cmd, "Name of the permission set license to assign.")
	return cmd
}

// The character classes of a generated password, and the classes each
// complexity level uses besides lower case letters.
const (
	passwordLower   = "abcdefghijklmnopqrstuvwxyz"
	passwordUpper   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	passwordNumbers = "1234567890"
	passwordSymbols = "!@#$%^&*()_[]|-"
)

var passwordClasses = [][]string{
	{},
	{passwordNumbers},
	{passwordSymbols},
	{passwordNumbers, passwordUpper},
	{passwordSymbols, passwordUpper},
	{passwordSymbols, passwordNumbers, passwordUpper},
}

// generatePassword builds a password the way sf does: one character of
// each class of the complexity level, lower case letters for the rest, in
// a random order.
func generatePassword(length, complexity int) (string, error) {
	pick := func(characters string) (byte, error) {
		index, err := rand.Int(rand.Reader, big.NewInt(int64(len(characters))))
		if err != nil {
			return 0, err
		}
		return characters[index.Int64()], nil
	}
	classes := append(append([]string{}, passwordClasses[complexity]...), passwordLower)
	password := make([]byte, 0, length)
	for i := 0; len(password) < length; i++ {
		class := passwordLower
		if i < len(classes) {
			class = classes[i]
		}
		character, err := pick(class)
		if err != nil {
			return "", err
		}
		password = append(password, character)
	}
	for i := len(password) - 1; i > 0; i-- {
		index, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return "", err
		}
		password[i], password[index.Int64()] = password[index.Int64()], password[i]
	}
	return string(password), nil
}

func newOrgGeneratePasswordCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "password",
		Short:       "Generate a random password for scratch org users.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "GenerateUserPasswordCommand"},
		RunE: func(cmd *cobra.Command, args []string) error {
			length, _ := cmd.Flags().GetInt("length")
			complexity, _ := cmd.Flags().GetInt("complexity")
			if err := integerInRange("complexity", complexity, 0, 5); err != nil {
				return err
			}
			if err := integerInRange("length", length, 8, 1000); err != nil {
				return err
			}
			if length < 20 {
				warn(cmd, "Starting in Summer '26, this command will fail if you specify a password length below 20. For now, the command is generating a password of length 20 instead of the requested length.")
				length = 20
			}
			if complexity < 3 {
				warn(cmd, "Starting in Summer '26, this command will fail if you specify a password complexity below 3. For now, the command is generating a password of complexity 3 instead of the requested complexity.")
				complexity = 3
			}
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			users, _ := cmd.Flags().GetStringArray("on-behalf-of")
			if len(users) == 0 {
				users = []string{org.Username}
			}
			results := make([]*orderedMap, 0, len(users))
			for _, user := range users {
				username := lib.ResolveSFDXUsername(user)
				// The user must have a local authorization, as in sf.
				if _, err := lib.ReadSFDXAuthFields(username); err != nil {
					notFound := newError("NamedOrgNotFoundError", "No authorization information found for %s.", user)
					if similar := similarOrgName(user); similar != "" {
						notFound.Actions = []string{fmt.Sprintf("It looks like you mistyped the username or alias. Did you mean %q?", similar)}
					}
					return notFound
				}
				id, err := userID(org, username)
				if err != nil {
					return err
				}
				password, err := generatePassword(length, complexity)
				if err != nil {
					return err
				}
				content, _ := json.Marshal(map[string]string{"NewPassword": password})
				if _, err := org.Force.PostREST("/sobjects/User/"+id+"/password", string(content)); err != nil {
					return apiError(err)
				}
				// The password is stored when the user has an
				// authorization file, so that "org auth
				// show-user-password" can print it.
				if _, err := lib.ReadSFDXAuthFields(username); err == nil {
					if err := lib.UpdateSFDXAuthFields(username, lib.SFDXAuthFields{"password": password}); err != nil {
						return err
					}
				}
				result := newOrderedMap()
				result.Set("username", username)
				result.Set("password", password)
				results = append(results, result)
			}
			warn(cmd, "This command has generated and displayed a password. Avoid sharing or logging this output as it contains a sensitive credential.")
			if len(results) == 1 {
				return emit(cmd, results[0], func(w io.Writer) {
					username := text(results[0].values["username"])
					fmt.Fprintln(w, paint(w, styleGreen, fmt.Sprintf("Successfully set the password \"%s\" for user %s.", text(results[0].values["password"]), username)))
					fmt.Fprintf(w, "You can see the password again by running \""+binName+" org auth show-user-password -o %s\".\n", username)
				})
			}
			return emit(cmd, results, func(w io.Writer) {
				rows := make([][]string, 0, len(results))
				for _, result := range results {
					rows = append(rows, []string{text(result.values["username"]), text(result.values["password"])})
				}
				fmt.Fprintln(w, paint(w, styleGreen, "Successfully set passwords:"))
				printTable(w, "", []string{"Username", "Password"}, rows)
			})
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().StringArrayP("on-behalf-of", "b", nil, "Comma-separated list of usernames or aliases to assign the password to; must have been created locally with the \"org create user\" command.")
	cmd.Flags().IntP("length", "l", 20, "Number of characters in the generated password; valid values are between 20 and 100.")
	cmd.Flags().IntP("complexity", "c", 5, "Level of password complexity or strength; the higher the value, the stronger the password.")
	addAPIVersionFlag(cmd)
	addTargetOrgFlag(cmd)
	return cmd
}

// orgUsernames returns the usernames sf knows for the org of an
// authorization: the ones recorded in ~/.sfdx/<orgId>.json, or the username
// of the authorization when there is no such file.
func orgUsernames(org *targetOrg) []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return []string{org.Username}
	}
	data, err := os.ReadFile(filepath.Join(home, ".sfdx", org.Fields.String("orgId")+".json"))
	if err != nil {
		return []string{org.Username}
	}
	var contents struct {
		Usernames []string `json:"usernames"`
	}
	if json.Unmarshal(data, &contents) != nil || contents.Usernames == nil {
		return []string{org.Username}
	}
	return contents.Usernames
}

func newOrgListUsersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "users",
		Short:       "List all locally-authenticated users of an org.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "ListUsersCommand"},
		RunE: func(cmd *cobra.Command, args []string) error {
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			users, err := org.Force.Query("SELECT username, profileid, id FROM User")
			if err != nil {
				return apiError(err)
			}
			profiles, err := org.Force.Query("SELECT id, name FROM Profile")
			if err != nil {
				return apiError(err)
			}
			profileNames := map[string]string{}
			for _, record := range profiles.Records {
				profileNames[text(record["Id"])] = text(record["Name"])
			}
			type userInfo struct{ id, profile string }
			infos := map[string]userInfo{}
			for _, record := range users.Records {
				infos[text(record["Username"])] = userInfo{id: text(record["Id"]), profile: text(record["ProfileId"])}
			}
			orgID := org.Fields.String("orgId")
			result := []*orderedMap{}
			rows := [][]string{}
			for _, username := range orgUsernames(org) {
				fields := org.Fields
				if username != org.Username {
					other, err := loadOrg(username)
					if err != nil {
						return newError("NamedOrgNotFoundError", "No authorization information found for %s.", username)
					}
					fields = other.Fields
				}
				alias := ""
				if aliases := aliasesFor(username); len(aliases) > 0 {
					alias = aliases[0]
				}
				marker := ""
				if username == org.Username {
					marker = "(A)"
				}
				info, known := infos[username]
				item := newOrderedMap()
				item.Set("defaultMarker", marker)
				item.Set("alias", alias)
				item.Set("username", username)
				profileName, hasProfile := profileNames[info.profile]
				if hasProfile {
					item.Set("profileName", profileName)
				}
				item.Set("orgId", orgID)
				item.Set("accessToken", redactedSecret())
				item.Set("instanceUrl", fields.String("instanceUrl"))
				item.Set("loginUrl", fields.String("loginUrl"))
				if known {
					item.Set("userId", info.id)
				}
				result = append(result, item)
				rows = append(rows, []string{marker, alias, username, profileName, info.id})
			}
			warnJSON(cmd, secretsHiddenWarning("org list users"))
			return emit(cmd, result, func(w io.Writer) {
				printTable(w, "Users in org "+orgID, []string{"Default", "Alias", "Username", "Profile Name", "User Id"}, rows)
			})
		},
	}
	addJSONFlag(cmd)
	addAPIVersionFlag(cmd)
	addTargetOrgFlag(cmd)
	return cmd
}
