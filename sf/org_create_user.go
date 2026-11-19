package sf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/spf13/cobra"

	"github.com/ForceCLI/force/lib"
)

func lowerFirst(word string) string {
	if word == "" {
		return word
	}
	runes := []rune(word)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

func upperFirst(word string) string {
	if word == "" {
		return word
	}
	runes := []rune(word)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

var roleDeveloperNamePattern = regexp.MustCompile(`(?i)^[a-z]\w*$`)

// singleRecordID returns the Id of the one record a query selects, or the
// error sf reports when there is none.
func singleRecordID(org *targetOrg, soql string) (string, error) {
	id, found, err := firstRecordID(org, soql)
	if err != nil {
		return "", err
	}
	if !found {
		return "", newError("SingleRecordQuery_NoRecords", "No record found for %s", soql)
	}
	return id, nil
}

// permissionSetID finds a permission set by name.  A name of the form
// ns__Name is a permission set in the namespace ns.
func permissionSetID(org *targetOrg, name string) (string, error) {
	escape := func(value string) string { return strings.ReplaceAll(value, "'", `\'`) }
	soql := fmt.Sprintf("SELECT Id FROM PermissionSet WHERE Name='%s'", escape(name))
	namespace, base, namespaced := strings.Cut(name, "__")
	if namespaced {
		soql = fmt.Sprintf("SELECT Id FROM PermissionSet WHERE Name='%s' AND NamespacePrefix='%s'", escape(base), escape(namespace))
	}
	id, found, err := firstRecordID(org, soql)
	if err != nil {
		return "", err
	}
	if !found {
		if namespaced {
			return "", fmt.Errorf("Permission set \"%s\" for namespace \"%s\" not found in target org. Do you need to push source or install the package?", base, namespace)
		}
		return "", fmt.Errorf("Permission set \"%s\" not found in target org. Do you need to push source?", name)
	}
	return id, nil
}

// permsetNames returns the permission sets named in the permsets field of
// a user definition: a list, or names separated by commas.
func permsetNames(value interface{}) []string {
	names := []string{}
	switch v := value.(type) {
	case []interface{}:
		for _, item := range v {
			names = append(names, fmt.Sprint(item))
		}
	case string:
		if v == "" {
			return names
		}
		for _, item := range strings.Split(v, ",") {
			names = append(names, strings.TrimSpace(strings.Replace(item, "'", "", 1)))
		}
	}
	return names
}

func isTrue(value interface{}) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(v, "true")
	}
	return false
}

// scimCreateUser creates a user with the SCIM API, the way sf does.  The
// org answers with the Id of the user and, in the auto-approve-user header,
// a refresh token for it.
func scimCreateUser(org *targetOrg, fields *orderedMap) (id string, refreshToken string, err error) {
	username := text(fields.values["username"])
	nickName := username
	if len(nickName) > 40 {
		nickName = nickName[:40]
	}
	body, _ := json.Marshal(map[string]interface{}{
		"username":     username,
		"emails":       []interface{}{fields.values["email"]},
		"name":         map[string]interface{}{"familyName": fields.values["lastName"]},
		"nickName":     nickName,
		"entitlements": []interface{}{map[string]interface{}{"value": fields.values["profileId"]}},
	})
	send := func() (*http.Response, []byte, error) {
		request, err := http.NewRequest("POST", strings.TrimRight(org.Force.Credentials.InstanceUrl, "/")+"/services/scim/v1/Users", bytes.NewReader(body))
		if err != nil {
			return nil, nil, err
		}
		request.Header.Set("Authorization", "Bearer "+org.Force.Credentials.AccessToken)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("auto-approve-user", "true")
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			return nil, nil, err
		}
		defer response.Body.Close()
		content, err := io.ReadAll(response.Body)
		return response, content, err
	}
	response, content, err := send()
	if err == nil && response.StatusCode == http.StatusUnauthorized {
		if err = org.Force.RefreshSession(); err == nil {
			response, content, err = send()
		}
	}
	if err != nil {
		return "", "", err
	}
	var parsed struct {
		ID     string `json:"id"`
		Errors []struct {
			Description string `json:"description"`
		}
	}
	json.Unmarshal(content, &parsed)
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated {
		message := fmt.Sprintf("An invalid http response code (%d) was received while trying to create a user.", response.StatusCode)
		if len(parsed.Errors) > 0 {
			message += " causes:\n"
			for _, problem := range parsed.Errors {
				message += "\n" + problem.Description
			}
		}
		return "", "", newError("UserCreateHttpError", "%s", message)
	}
	return parsed.ID, response.Header.Get("auto-approve-user"), nil
}

// addOrgUsername records a username in ~/.sfdx/<orgId>.json, where sf
// keeps the users it created for an org.
func addOrgUsername(org *targetOrg, username string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	usernames := orgUsernames(org)
	if containsString(usernames, username) {
		return nil
	}
	data, err := json.MarshalIndent(map[string][]string{"usernames": append(usernames, username)}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(home, ".sfdx", org.Fields.String("orgId")+".json"), data, 0o600)
}

func newOrgCreateUserCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "user",
		Short:       "Create a user for a scratch org.",
		Args:        cobra.ArbitraryArgs,
		Annotations: map[string]string{commandNameAnnotation: "CreateUserCommand"},
		RunE: func(cmd *cobra.Command, args []string) error {
			varargs, err := nameValueArguments(args)
			if err != nil {
				return err
			}
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			if org.Fields["isScratch"] != true && org.Fields.String("devHubUsername") == "" {
				return newError("NonScratchOrgError", "This command works with only scratch orgs.")
			}
			if org.Fields.String("privateKey") != "" {
				return newError("UnsupportedAuthError", "The sf-compatible mode of force creates users only for an org that was authorized with a refresh token.")
			}
			escape := func(value string) string { return strings.ReplaceAll(value, "'", `\'`) }

			// The new user starts as a copy of the user of the target org
			// with the Standard User profile.
			admin, err := org.Force.Query(fmt.Sprintf("SELECT Id, Username, LastName, Alias, TimeZoneSidKey, LocaleSidKey, EmailEncodingKey, ProfileId, LanguageLocaleKey, Email FROM User WHERE Username='%s'", escape(org.Username)))
			if err != nil {
				return apiError(err)
			}
			if len(admin.Records) != 1 {
				return newError("UserQueryFailedError", "Failed to query for the user %s.", org.Username)
			}
			record := admin.Records[0]
			standardProfile, err := singleRecordID(org, "SELECT Id FROM Profile WHERE name='Standard User'")
			if err != nil {
				return err
			}
			defaultUsername := fmt.Sprintf("%d_%s", time.Now().UnixMilli(), org.Username)
			fields := newOrderedMap()
			fields.Set("id", record["Id"])
			fields.Set("username", defaultUsername)
			fields.Set("alias", record["Alias"])
			fields.Set("email", record["Email"])
			fields.Set("emailEncodingKey", record["EmailEncodingKey"])
			fields.Set("languageLocaleKey", record["LanguageLocaleKey"])
			fields.Set("localeSidKey", record["LocaleSidKey"])
			fields.Set("profileId", standardProfile)
			fields.Set("lastName", record["LastName"])
			fields.Set("timeZoneSidKey", record["TimeZoneSidKey"])

			// The definition file overrides the defaults, and the
			// arguments override the file.
			if path, _ := cmd.Flags().GetString("definition-file"); path != "" {
				data, err := os.ReadFile(path)
				if err != nil {
					return fileSystemError("open", path, err)
				}
				parsed, err := parseOrderedJSON(data)
				if err != nil {
					return err
				}
				definition, ok := parsed.(*orderedMap)
				if !ok {
					return fmt.Errorf("%s: the user definition is not a JSON object", path)
				}
				for _, key := range definition.keys {
					fields.Set(lowerFirst(key), definition.values[key])
				}
			}
			for _, key := range varargs.keys {
				switch strings.ToLower(key) {
				case "generatepassword":
					fields.Set("generatePassword", isTrue(varargs.values[key]))
				case "profilename":
					fields.Set("profileName", varargs.values[key])
				case "roledevelopername":
					fields.Set("roleDeveloperName", varargs.values[key])
				default:
					fields.Set(lowerFirst(key), varargs.values[key])
				}
			}
			orgID := org.Fields.String("orgId")
			if unique, _ := cmd.Flags().GetBool("set-unique-username"); unique && fields.values["username"] != defaultUsername {
				fields.Set("username", fmt.Sprintf("%v.%s", fields.values["username"], strings.ToLower(orgID)))
			}
			if name := text(fields.values["profileName"]); name != "" {
				id, err := singleRecordID(org, fmt.Sprintf("SELECT id FROM profile WHERE name='%s'", name))
				if err != nil {
					return err
				}
				fields.Set("profileId", id)
			}
			if name := text(fields.values["roleDeveloperName"]); name != "" {
				if !roleDeveloperNamePattern.MatchString(name) || strings.Contains(name, "__") || strings.HasSuffix(name, "_") {
					return newError("InvalidRoleDeveloperNameError", "Invalid roleDeveloperName: \"%s\". Must start with a letter and contain only alphanumeric characters or single underscores, with no double or final underscores.", name)
				}
				id, err := singleRecordID(org, fmt.Sprintf("SELECT id FROM userrole WHERE developername='%s'", name))
				if err != nil {
					return err
				}
				fields.Set("userRoleId", id)
			}
			username := text(fields.values["username"])

			userID, refreshToken, err := scimCreateUser(org, fields)
			if err != nil {
				message := err.Error()
				switch {
				case strings.Contains(message, "LICENSE_LIMIT_EXCEEDED"):
					profileName := text(fields.values["profileId"])
					if result, queryErr := org.Force.Query(fmt.Sprintf("SELECT name FROM profile WHERE id='%s'", profileName)); queryErr == nil && len(result.Records) > 0 {
						profileName = text(result.Records[0]["Name"])
					}
					return newError("licenseLimitExceeded", "There are no available user licenses for the user profile \"%s\".", profileName)
				case strings.Contains(message, "DUPLICATE_USERNAME"):
					return newError("duplicateUsername", "The username \"%s\" already exists in this or another Salesforce org. Usernames must be unique across all Salesforce orgs. Try using the --set-unique-username flag to force a unique username by appending the org ID.", username)
				}
				return err
			}

			// The SCIM request sets the username, email, last name and
			// profile; the other fields are set with an update.
			update := newOrderedMap()
			for _, key := range fields.keys {
				switch key {
				case "id", "username", "email", "lastName", "profileId", "permsets", "generatepassword", "generatePassword", "profileName", "roleDeveloperName":
				default:
					update.Set(upperFirst(key), fields.values[key])
				}
			}
			content, _ := json.Marshal(update)
			if _, err := org.Force.PatchREST("/sobjects/User/"+userID, string(content)); err != nil {
				return apiError(err)
			}

			// Authorize the new user with the refresh token the org gave,
			// check that the authorization works, and save it.
			instanceURL := strings.TrimRight(org.Fields.String("instanceUrl"), "/")
			session := lib.ForceSession{
				InstanceUrl:    instanceURL,
				EndpointUrl:    instanceURL,
				RefreshToken:   refreshToken,
				ClientId:       org.Fields.String("clientId"),
				SessionOptions: &lib.SessionOptions{},
			}
			userForce := lib.NewForce(&session)
			if err := userForce.RefreshAccessToken(); err != nil {
				return apiError(err)
			}
			if _, err := userForce.GetREST("/sobjects/User/describe"); err != nil {
				return apiError(err)
			}
			auth := lib.SFDXAuthFields{
				"orgId":                           orgID,
				"username":                        username,
				"accessToken":                     userForce.Credentials.AccessToken,
				"instanceUrl":                     instanceURL,
				"loginUrl":                        instanceURL,
				"refreshToken":                    refreshToken,
				"clientId":                        org.Fields.String("clientId"),
				"isDevHub":                        false,
				"instanceApiVersion":              lib.ApiVersionNumber(),
				"instanceApiVersionLastRetrieved": time.Now().Format("1/2/2006, 3:04:05 PM"),
				"userId":                          userID,
			}
			order := []string{"orgId", "username", "accessToken", "instanceUrl", "loginUrl", "refreshToken", "clientId", "isDevHub", "instanceApiVersion", "instanceApiVersionLastRetrieved", "userId", "userProfileName", "password"}
			if name := text(fields.values["profileName"]); name != "" {
				auth["userProfileName"] = name
			}
			if err := lib.WriteSFDXAuthFields(auth, order...); err != nil {
				return err
			}
			if err := addOrgUsername(org, username); err != nil {
				return err
			}

			var successes, failures []assignment
			permsets := permsetNames(fields.values["permsets"])
			if _, given := fields.values["permsets"]; given && len(permsets) > 0 {
				var assignErr error
				for _, name := range permsets {
					var id string
					if id, assignErr = permissionSetID(org, name); assignErr != nil {
						break
					}
					record := newOrderedMap()
					record.Set("AssigneeId", userID)
					record.Set("PermissionSetId", id)
					if assignErr = createRecord(org, "PermissionSetAssignment", record); assignErr != nil {
						break
					}
				}
				if assignErr != nil {
					failures = append(failures, assignment{name: "Permission Set Assignment", value: assignErr.Error()})
				} else {
					successes = append(successes, assignment{name: "Permission Set Assignment", value: strings.Join(permsets, ",")})
				}
			}
			if isTrue(fields.values["generatePassword"]) {
				password, err := generatePassword(13, 5)
				if err == nil {
					body, _ := json.Marshal(map[string]string{"NewPassword": password})
					_, err = org.Force.PostREST("/sobjects/User/"+userID+"/password", string(body))
				}
				if err == nil {
					err = lib.UpdateSFDXAuthFields(username, lib.SFDXAuthFields{"password": password})
				}
				if err != nil {
					failures = append(failures, assignment{name: "Password Assignment", value: err.Error()})
				} else {
					successes = append(successes, assignment{name: "Password Assignment", value: password})
					warn(cmd, "This command has generated and displayed a password. Avoid sharing or logging this output as it contains a sensitive credential.")
				}
			}
			if alias, _ := cmd.Flags().GetString("set-alias"); alias != "" {
				if err := lib.SetSFDXAlias(alias, username); err != nil {
					return err
				}
			}
			fields.Set("id", userID)

			resultFields := newOrderedMap()
			for _, key := range fields.keys {
				if key != "permsets" {
					resultFields.Set(strings.ToLower(key), fields.values[key])
				}
			}
			result := newOrderedMap()
			result.Set("orgId", orgID)
			result.Set("permissionSetAssignments", permsets)
			result.Set("fields", resultFields)
			status := 0
			switch {
			case len(failures) > 0 && len(successes) > 0:
				status = 68
			case len(failures) > 0:
				status = 1
			}
			if err := emitStatus(cmd, status, result, func(w io.Writer) {
				created := fmt.Sprintf("Successfully created user \"%s\" with ID %s for org %s.\n\nSee more details about this user by running \""+binName+" org user display -o %s\".", username, userID, orgID, username)
				if len(failures) == 0 {
					fmt.Fprintln(w, created)
					return
				}
				fmt.Fprintln(w, "=== Partial Success")
				fmt.Fprintln(w)
				fmt.Fprintln(w, created)
				fmt.Fprintln(w)
				fmt.Fprintln(w, "=== Failures")
				fmt.Fprintln(w)
				rows := make([][]string, 0, len(failures))
				for _, failure := range failures {
					rows = append(rows, []string{failure.name, failure.value})
				}
				printTable(w, "", []string{"Action", "Error Message"}, rows)
			}); err != nil {
				return err
			}
			if status != 0 {
				return exitStatus(status)
			}
			return nil
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().StringP("set-alias", "a", "", "Set an alias for the created username to reference in other CLI commands.")
	cmd.Flags().StringP("definition-file", "f", "", "File path to a user definition file for customizing the new user.")
	cmd.Flags().BoolP("set-unique-username", "s", false, "Force the username, if specified in the definition file or at the command line, to be unique by appending the org ID.")
	addAPIVersionFlag(cmd)
	addTargetOrgFlag(cmd)
	return cmd
}
