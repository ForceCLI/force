package lib

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"time"
)

type ScratchOrg struct {
	UserName    string
	InstanceUrl string
	AuthCode    string
}

type AuthCodeSession struct {
	ForceSession
	RefreshToken string `json:"refresh_token"`
}

func (s *ScratchOrg) tokenURL() string {
	return fmt.Sprintf("%s/services/oauth2/token", s.InstanceUrl)
}

func (f *Force) getScratchOrg(scratchOrgId string) (scratchOrg ScratchOrg, err error) {
	org, err := f.waitForScratchOrgReady(scratchOrgId)
	if err != nil {
		return
	}
	username, ok := org["SignupUsername"].(string)
	if !ok || username == "" {
		err = errors.New("Scratch org is not ready: SignupUsername is not available")
		return
	}
	loginUrl, ok := org["LoginUrl"].(string)
	if !ok || loginUrl == "" {
		err = errors.New("Scratch org is not ready: LoginUrl is not available")
		return
	}
	authCode, ok := org["AuthCode"].(string)
	if !ok || authCode == "" {
		err = errors.New("Scratch org is not ready: AuthCode is not available")
		return
	}
	scratchOrg = ScratchOrg{
		UserName:    username,
		InstanceUrl: loginUrl,
		AuthCode:    authCode,
	}
	return
}

var scratchOrgPollInterval = 5 * time.Second
var scratchOrgMaxWait = 10 * time.Minute

func (f *Force) waitForScratchOrgReady(scratchOrgId string) (org ForceRecord, err error) {
	start := time.Now()
	for {
		org, err = f.GetRecord("ScratchOrgInfo", scratchOrgId)
		if err != nil {
			err = errors.New("Unable to query scratch orgs.  You must be logged into a Dev Hub org.")
			return
		}

		status, _ := org["Status"].(string)
		switch status {
		case "Active":
			return org, nil
		case "Error":
			errorCode, _ := org["ErrorCode"].(string)
			if errorCode != "" {
				err = fmt.Errorf("Scratch org creation failed: %s", errorCode)
			} else {
				err = errors.New("Scratch org creation failed")
			}
			return
		case "New":
			if time.Since(start) > scratchOrgMaxWait {
				err = errors.New("Timed out waiting for scratch org to become ready")
				return
			}
			fmt.Fprintf(os.Stderr, "Waiting for scratch org to be ready (status: %s)...\n", status)
			time.Sleep(scratchOrgPollInterval)
		default:
			if time.Since(start) > scratchOrgMaxWait {
				err = fmt.Errorf("Timed out waiting for scratch org (status: %s)", status)
				return
			}
			fmt.Fprintf(os.Stderr, "Waiting for scratch org to be ready (status: %s)...\n", status)
			time.Sleep(scratchOrgPollInterval)
		}
	}
}

// Log into a Scratch Org
func (f *Force) ForceLoginNewScratch(scratchOrgId string) (session ForceSession, err error) {
	scratchOrg, err := f.getScratchOrg(scratchOrgId)
	if err != nil {
		return
	}
	session, err = scratchOrg.getSession()
	if err != nil {
		return
	}
	session.SessionOptions = &SessionOptions{
		RefreshMethod: RefreshOauth,
	}
	session.ForceEndpoint = EndpointTest
	session.ClientId = "PlatformCLI"
	return
}

// Use the AuthCode for a new Scratch Org from the Dev Hub to get a new
// ForceSession
func (s *ScratchOrg) getSession() (session ForceSession, err error) {
	attrs := url.Values{}
	attrs.Set("grant_type", "authorization_code")
	attrs.Set("code", s.AuthCode)
	attrs.Set("client_id", "PlatformCLI")
	attrs.Set("redirect_uri", "http://localhost:1717/OauthRedirect")

	postVars := attrs.Encode()
	req, err := httpRequest("POST", s.tokenURL(), bytes.NewReader([]byte(postVars)))
	if err != nil {
		return
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	fmt.Fprintln(os.Stderr, "Logging into scratch org")
	res, err := doRequest(req)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if res.StatusCode != 200 {
		var oauthError OAuthError
		json.Unmarshal(body, &oauthError)
		err = errors.New("Unable to log into scratch org: " + oauthError.ErrorDescription)
		return
	}
	if err != nil {
		return
	}
	var authSession AuthCodeSession

	err = json.Unmarshal(body, &authSession)
	if err != nil {
		return
	}

	session = authSession.ForceSession
	session.RefreshToken = authSession.RefreshToken

	return session, nil
}

// DeploySettings deploys settings metadata to a scratch org
func (f *Force) DeploySettings(settings []string) error {
	files := buildSettingsMetadata(settings)

	// Deploy the metadata
	options := ForceDeployOptions{}
	result, err := f.Metadata.Deploy(files, options)
	if err != nil {
		return fmt.Errorf("settings deployment failed: %w", err)
	}

	// Check if deployment succeeded
	if !result.Success {
		errorMsg := fmt.Sprintf("Settings deployment failed with status: %s", result.Status)
		if result.ErrorMessage != "" {
			errorMsg += fmt.Sprintf(": %s", result.ErrorMessage)
		}
		if len(result.Details.ComponentFailures) > 0 {
			errorMsg += "\nComponent failures:"
			for _, failure := range result.Details.ComponentFailures {
				errorMsg += fmt.Sprintf("\n  - %s: %s", failure.FileName, failure.Problem)
			}
		}
		return errors.New(errorMsg)
	}

	return nil
}

// settingsFile builds a settings metadata XML file with the given root
// element and the requested true preferences. Returns nil if no preferences
// are enabled. Preferences are emitted in the order they appear in flags.
func settingsFile(rootElement string, flags []settingsFlag) []byte {
	any := false
	for _, f := range flags {
		if f.enabled {
			any = true
			break
		}
	}
	if !any {
		return nil
	}
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<`)
	buf.WriteString(rootElement)
	buf.WriteString(` xmlns="http://soap.sforce.com/2006/04/metadata">
`)
	for _, f := range flags {
		if f.enabled {
			fmt.Fprintf(&buf, "    <%s>true</%s>\n", f.name, f.name)
		}
	}
	buf.WriteString(`</`)
	buf.WriteString(rootElement)
	buf.WriteString(`>`)
	return buf.Bytes()
}

type settingsFlag struct {
	name    string
	enabled bool
}

func buildSettingsMetadata(settings []string) ForceMetadataFiles {
	files := make(ForceMetadataFiles)

	// Create package.xml at root of zip
	packageXml := `<?xml version="1.0" encoding="UTF-8"?>
<Package xmlns="http://soap.sforce.com/2006/04/metadata">
    <types>
        <members>*</members>
        <name>Settings</name>
    </types>
    <version>64.0</version>
</Package>`
	files["unpackaged/package.xml"] = []byte(packageXml)

	// Track each preference individually so multiple flags can share one file.
	var (
		enableEnhancedNotes               bool
		enableTasksOnEnhancedNotes        bool
		enableQuote                       bool
		enableQuotesWithoutOppEnabled     bool
		networksEnabled                   bool
		commerceEnabled                   bool
		enableOrders                      bool
		enableEnhancedCommerceOrders      bool
		enableOrderEvents                 bool
		enableOptionalPricebook           bool
		enableZeroQuantity                bool
		enableNegativeQuantity            bool
		enableApexApprovalLockUnlock      bool
		permsetsInFieldCreation           bool
		enableLightningPreviewPref        bool
		enableS1DesktopEnabled            bool
		enableLiveAgent                   bool
		enableMultiCurrency               bool
		enableCoreCPQ                     bool
		enableSubscriptionManagement      bool
		enableKnowledge                   bool
		enableLightningKnowledge          bool
		enableBillingSetup                bool
		enableExperienceBundleMetadata    bool
		enableContextDefinitions          bool
		enableEinsteinGptPlatform         bool
		enableOpportunityTeam             bool
		enableOrderManagement             bool
		enableHighAvailability            bool
		enablePricingWaterfall            bool
		enablePricingWaterfallPersistence bool
		enableSalesforcePricing           bool
		enableRating                      bool
		enableRatingWaterfall             bool
		enableRatingWaterfallPersistence  bool
		enableProductConfigurator         bool
		enableDFOPref                     bool
	)

	for _, setting := range settings {
		switch setting {
		case "enableEnhancedNotes":
			enableEnhancedNotes = true
			enableTasksOnEnhancedNotes = true
		case "enableQuote":
			enableQuote = true
		case "enableQuotesWithoutOppEnabled":
			enableQuotesWithoutOppEnabled = true
		case "networksEnabled":
			networksEnabled = true
		case "commerceEnabled":
			commerceEnabled = true
		case "enableOrders", "enableEnhancedCommerceOrders":
			enableOrders = true
			enableEnhancedCommerceOrders = true
		case "enableOrderEvents":
			enableOrderEvents = true
		case "enableOptionalPricebook":
			enableOptionalPricebook = true
		case "enableZeroQuantity":
			enableZeroQuantity = true
		case "enableNegativeQuantity":
			enableNegativeQuantity = true
		case "enableApexApprovalLockUnlock":
			enableApexApprovalLockUnlock = true
		case "permsetsInFieldCreation":
			permsetsInFieldCreation = true
		case "enableLightningPreviewPref":
			enableLightningPreviewPref = true
		case "enableS1DesktopEnabled":
			enableS1DesktopEnabled = true
		case "enableLiveAgent":
			enableLiveAgent = true
		case "enableMultiCurrency":
			enableMultiCurrency = true
		case "enableCoreCPQ":
			enableCoreCPQ = true
		case "enableSubscriptionManagement":
			enableSubscriptionManagement = true
		case "enableKnowledge":
			enableKnowledge = true
		case "enableLightningKnowledge":
			enableLightningKnowledge = true
		case "enableBillingSetup":
			enableBillingSetup = true
		case "enableExperienceBundleMetadata":
			enableExperienceBundleMetadata = true
		case "enableContextDefinitions":
			enableContextDefinitions = true
		case "enableEinsteinGptPlatform":
			enableEinsteinGptPlatform = true
		case "enableOpportunityTeam":
			enableOpportunityTeam = true
		case "enableOrderManagement":
			enableOrderManagement = true
		case "enableHighAvailability":
			enableHighAvailability = true
		case "enablePricingWaterfall":
			enablePricingWaterfall = true
		case "enablePricingWaterfallPersistence":
			enablePricingWaterfallPersistence = true
		case "enableSalesforcePricing":
			enableSalesforcePricing = true
		case "enableRating":
			enableRating = true
		case "enableRatingWaterfall":
			enableRatingWaterfall = true
		case "enableRatingWaterfallPersistence":
			enableRatingWaterfallPersistence = true
		case "enableProductConfigurator":
			enableProductConfigurator = true
		case "enableDFOPref":
			enableDFOPref = true
		}
	}

	emit := func(path, root string, flags []settingsFlag) {
		if content := settingsFile(root, flags); content != nil {
			files[path] = content
		}
	}

	emit("unpackaged/settings/EnhancedNotes.settings", "EnhancedNotesSettings", []settingsFlag{
		{"enableEnhancedNotes", enableEnhancedNotes},
		{"enableTasksOnEnhancedNotes", enableTasksOnEnhancedNotes},
	})
	emit("unpackaged/settings/Quote.settings", "QuoteSettings", []settingsFlag{
		{"enableQuote", enableQuote},
		{"enableQuotesWithoutOppEnabled", enableQuotesWithoutOppEnabled},
	})
	emit("unpackaged/settings/Communities.settings", "CommunitiesSettings", []settingsFlag{
		{"enableNetworksEnabled", networksEnabled},
	})
	emit("unpackaged/settings/Commerce.settings", "CommerceSettings", []settingsFlag{
		{"commerceEnabled", commerceEnabled},
	})
	emit("unpackaged/settings/Order.settings", "OrderSettings", []settingsFlag{
		{"enableOrders", enableOrders},
		{"enableEnhancedCommerceOrders", enableEnhancedCommerceOrders},
		{"enableOrderEvents", enableOrderEvents},
		{"enableOptionalPricebook", enableOptionalPricebook},
		{"enableZeroQuantity", enableZeroQuantity},
		{"enableNegativeQuantity", enableNegativeQuantity},
	})
	emit("unpackaged/settings/Apex.settings", "ApexSettings", []settingsFlag{
		{"enableApexApprovalLockUnlock", enableApexApprovalLockUnlock},
	})
	emit("unpackaged/settings/UserManagement.settings", "UserManagementSettings", []settingsFlag{
		{"permsetsInFieldCreation", permsetsInFieldCreation},
	})
	emit("unpackaged/settings/LightningExperience.settings", "LightningExperienceSettings", []settingsFlag{
		{"enableLightningPreviewPref", enableLightningPreviewPref},
		{"enableS1DesktopEnabled", enableS1DesktopEnabled},
	})
	emit("unpackaged/settings/LiveAgent.settings", "LiveAgentSettings", []settingsFlag{
		{"enableLiveAgent", enableLiveAgent},
	})
	emit("unpackaged/settings/Currency.settings", "CurrencySettings", []settingsFlag{
		{"enableMultiCurrency", enableMultiCurrency},
	})
	emit("unpackaged/settings/RevenueManagement.settings", "RevenueManagementSettings", []settingsFlag{
		{"enableCoreCPQ", enableCoreCPQ},
	})
	emit("unpackaged/settings/SubscriptionManagement.settings", "SubscriptionManagementSettings", []settingsFlag{
		{"enableSubscriptionManagement", enableSubscriptionManagement},
	})
	emit("unpackaged/settings/Knowledge.settings", "KnowledgeSettings", []settingsFlag{
		{"enableKnowledge", enableKnowledge},
		{"enableLightningKnowledge", enableLightningKnowledge},
	})
	emit("unpackaged/settings/Billing.settings", "BillingSettings", []settingsFlag{
		{"enableBillingSetup", enableBillingSetup},
	})
	emit("unpackaged/settings/ExperienceBundle.settings", "ExperienceBundleSettings", []settingsFlag{
		{"enableExperienceBundleMetadata", enableExperienceBundleMetadata},
	})
	emit("unpackaged/settings/IndustriesContext.settings", "IndustriesContextSettings", []settingsFlag{
		{"enableContextDefinitions", enableContextDefinitions},
	})
	emit("unpackaged/settings/EinsteinGpt.settings", "EinsteinGptSettings", []settingsFlag{
		{"enableEinsteinGptPlatform", enableEinsteinGptPlatform},
	})
	emit("unpackaged/settings/Opportunity.settings", "OpportunitySettings", []settingsFlag{
		{"enableOpportunityTeam", enableOpportunityTeam},
	})
	emit("unpackaged/settings/OrderManagement.settings", "OrderManagementSettings", []settingsFlag{
		{"enableOrderManagement", enableOrderManagement},
	})
	emit("unpackaged/settings/IndustriesPricing.settings", "IndustriesPricingSettings", []settingsFlag{
		{"enableHighAvailability", enableHighAvailability},
		{"enablePricingWaterfall", enablePricingWaterfall},
		{"enablePricingWaterfallPersistence", enablePricingWaterfallPersistence},
		{"enableSalesforcePricing", enableSalesforcePricing},
	})
	emit("unpackaged/settings/IndustriesRating.settings", "IndustriesRatingSettings", []settingsFlag{
		{"enableRating", enableRating},
		{"enableRatingWaterfall", enableRatingWaterfall},
		{"enableRatingWaterfallPersistence", enableRatingWaterfallPersistence},
	})
	emit("unpackaged/settings/ProductConfigurator.settings", "ProductConfiguratorSettings", []settingsFlag{
		{"enableProductConfigurator", enableProductConfigurator},
	})
	emit("unpackaged/settings/DynamicFulfillmentOrchestrator.settings", "DynamicFulfillmentOrchestratorSettings", []settingsFlag{
		{"enableDFOPref", enableDFOPref},
	})

	return files
}

// Create a new Scratch Org from a Dev Hub Org
func (f *Force) CreateScratchOrg() (id string, err error) {
	return f.CreateScratchOrgWithUser("")
}

func (f *Force) CreateScratchOrgWithUser(username string) (id string, err error) {
	return f.CreateScratchOrgWithUserAndFeatures(username, []string{})
}

func (f *Force) CreateScratchOrgWithUserAndFeatures(username string, features []string) (id string, err error) {
	return f.CreateScratchOrgWithUserFeaturesAndEdition(username, features, "")
}

func (f *Force) CreateScratchOrgWithUserFeaturesAndEdition(username string, features []string, edition string) (id string, err error) {
	return f.CreateScratchOrgWithUserFeaturesEditionAndSettings(username, features, edition, []string{})
}

func (f *Force) CreateScratchOrgWithUserFeaturesEditionAndSettings(username string, features []string, edition string, settings []string) (id string, err error) {
	return f.CreateScratchOrgWithUserFeaturesEditionSettingsAndNamespace(username, features, edition, settings, "")
}

func (f *Force) CreateScratchOrgWithUserFeaturesEditionSettingsAndNamespace(username string, features []string, edition string, settings []string, namespace string) (id string, err error) {
	return f.CreateScratchOrgWithRelease(username, features, edition, settings, namespace, "")
}

func (f *Force) CreateScratchOrgWithRelease(username string, features []string, edition string, settings []string, namespace string, release string) (id string, err error) {
	return f.CreateScratchOrgWithDuration(username, features, edition, settings, namespace, release, 7)
}

func (f *Force) CreateScratchOrgWithDuration(username string, features []string, edition string, settings []string, namespace string, release string, duration int) (id string, err error) {
	params := make(map[string]string)
	params["ConnectedAppCallbackUrl"] = "http://localhost:1717/OauthRedirect"
	params["ConnectedAppConsumerKey"] = "PlatformCLI"
	params["Country"] = "US"
	params["DurationDays"] = fmt.Sprintf("%d", duration)

	if edition != "" {
		params["Edition"] = edition
	} else {
		params["Edition"] = "Developer"
	}

	baseFeatures := "AuthorApex;API;AddCustomApps:30;AddCustomTabs:30;ForceComPlatform;Sites;CustomerSelfService"
	if len(features) > 0 {
		for _, feature := range features {
			baseFeatures += ";" + feature
		}
	}
	params["Features"] = baseFeatures

	// Note: settings parameter is kept for later deployment after org creation

	params["OrgName"] = "Force CLI Scratch"
	if namespace != "" {
		params["Namespace"] = namespace
	}
	if username != "" {
		params["Username"] = username
	}
	if release != "" {
		params["Release"] = release
	}
	id, err, messages := f.CreateRecord("ScratchOrgInfo", params)
	if err != nil {
		if len(messages) == 1 && messages[0].ErrorCode == "NOT_FOUND" {
			return "", DevHubOrgRequiredError
		}
		return
	}
	return
}
