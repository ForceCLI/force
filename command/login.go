package command

import (
	"errors"
	"fmt"
	"net/url"
	"os"

	"github.com/bgentry/speakeasy"
	"github.com/spf13/cobra"
	"github.com/thediveo/enumflag/v2"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
)

type ScratchFeature enumflag.Flag

const (
	PersonAccounts ScratchFeature = iota
	B2BCommerce
	ContactsToMultipleAccounts
	FinancialServicesUser
	StateAndCountryPicklist
	Communities
	HealthCloudAddOn
	HealthCloudUser
	ApexUserModeWithPermset
	EventLogFile
	AnalyticsAdminPerms
	WavePlatform
	InsightsPlatform
	EinsteinAnalyticsPlus
	EinsteinBuilderFree
	DevelopmentWave
	PlatformCache
	ServiceCloud
	ScvMultipartyAndConsult
	ServiceCloudVoicePartnerTelephony
	OrderManagement
	LiveAgent
	PlatformEncryption
	RevSubscriptionManagement
	CoreCpq
	Knowledge
	DSARPortability
	ProgramManagement
	EnableSetPasswordInApi
	OrderSaveLogicEnabled
	PartnerCommunity
	CustomerCommunityPlus
	UsageManagement
	BillingAdvanced
	DocGen
	Einstein1AIPlatform
	EmbeddedServiceMessaging
	BYOOTT
	LiveMessage
	Enablement
	SurveyAdvancedFeatures
	Slack
	EducationCloud
	AccountingSubledgerGrowthEdition
	AccountingSubledgerUser
	AdmissionsConnectUser
	AdvisorLinkFeature
	AdvisorLinkPathwaysFeature
	AnalyticsQueryService
	Assessments
	DataProcessingEngine
	DecisionTable
	DocGenDesigner
	DocGenInd
	DocumentChecklist
	FlowSites
	Fundraising
	IndustriesActionPlan
	IndustriesSalesExcellenceAddOn
	IndustriesServiceExcellenceAddOn
	LightningScheduler
	LightningServiceConsole
	MarketingUser
	OmniStudioDesigner
	OmniStudioRuntime
	PublicSectorAccess
	SharedActivities
	HighVelocitySales
	InvoiceManagement
	WorkplaceCommandCenterUser
	ForceComPlatform
	Division
	Grantmaking
	OutcomeManagement
	NonprofitCloudCaseManagementUser
	VolunteerManagement
	FinancialServicesInsuranceUser
	FSCAlertFramework
	FSCServiceProcess
	IndustriesBranchManagement
	IndustriesCompliantDataSharing
	InsuranceCalculationUser
	InsuranceClaimMgmt
	InsurancePolicyAdmin
	BusinessRulesEngine
	ApexIntegrationTests
	IndustriesManufacturingCmty
	IndustriesMfgAccountForecast
	IndustriesMfgAdvncdAccFrcs
	IndustriesMfgPartnerVisitMgmt
	IndustriesMfgProgram
	IndustriesMfgRebates
	IndustriesMfgTargets
	EnablePRM
	LifeSciencesCloudForCustomerEngagement
)

var ScratchFeatureIds = map[ScratchFeature][]string{
	PersonAccounts:                         {"PersonAccounts"},
	B2BCommerce:                            {"B2BCommerce"},
	ContactsToMultipleAccounts:             {"ContactsToMultipleAccounts"},
	FinancialServicesUser:                  {"FinancialServicesUser"},
	StateAndCountryPicklist:                {"StateAndCountryPicklist"},
	Communities:                            {"Communities"},
	HealthCloudAddOn:                       {"HealthCloudAddOn"},
	HealthCloudUser:                        {"HealthCloudUser"},
	ApexUserModeWithPermset:                {"ApexUserModeWithPermset"},
	EventLogFile:                           {"EventLogFile"},
	AnalyticsAdminPerms:                    {"AnalyticsAdminPerms"},
	WavePlatform:                           {"WavePlatform"},
	InsightsPlatform:                       {"InsightsPlatform"},
	EinsteinAnalyticsPlus:                  {"EinsteinAnalyticsPlus"},
	EinsteinBuilderFree:                    {"EinsteinBuilderFree"},
	DevelopmentWave:                        {"DevelopmentWave"},
	PlatformCache:                          {"PlatformCache"},
	ServiceCloud:                           {"ServiceCloud"},
	ScvMultipartyAndConsult:                {"ScvMultipartyAndConsult"},
	ServiceCloudVoicePartnerTelephony:      {"ServiceCloudVoicePartnerTelephony"},
	OrderManagement:                        {"OrderManagement"},
	LiveAgent:                              {"LiveAgent"},
	PlatformEncryption:                     {"PlatformEncryption"},
	RevSubscriptionManagement:              {"RevSubscriptionManagement"},
	CoreCpq:                                {"CoreCpq"},
	Knowledge:                              {"Knowledge"},
	DSARPortability:                        {"DSARPortability"},
	ProgramManagement:                      {"ProgramManagement"},
	EnableSetPasswordInApi:                 {"EnableSetPasswordInApi"},
	OrderSaveLogicEnabled:                  {"OrderSaveLogicEnabled"},
	PartnerCommunity:                       {"PartnerCommunity"},
	CustomerCommunityPlus:                  {"CustomerCommunityPlus"},
	UsageManagement:                        {"UsageManagement"},
	BillingAdvanced:                        {"BillingAdvanced"},
	DocGen:                                 {"DocGen"},
	Einstein1AIPlatform:                    {"Einstein1AIPlatform"},
	EmbeddedServiceMessaging:               {"EmbeddedServiceMessaging"},
	BYOOTT:                                 {"BYOOTT"},
	LiveMessage:                            {"LiveMessage"},
	Enablement:                             {"Enablement"},
	SurveyAdvancedFeatures:                 {"SurveyAdvancedFeatures"},
	Slack:                                  {"Slack"},
	EducationCloud:                         {"EducationCloud"},
	AccountingSubledgerGrowthEdition:       {"AccountingSubledgerGrowthEdition"},
	AccountingSubledgerUser:                {"AccountingSubledgerUser"},
	AdmissionsConnectUser:                  {"AdmissionsConnectUser"},
	AdvisorLinkFeature:                     {"AdvisorLinkFeature"},
	AdvisorLinkPathwaysFeature:             {"AdvisorLinkPathwaysFeature"},
	AnalyticsQueryService:                  {"AnalyticsQueryService"},
	Assessments:                            {"Assessments"},
	DataProcessingEngine:                   {"DataProcessingEngine"},
	DecisionTable:                          {"DecisionTable"},
	DocGenDesigner:                         {"DocGenDesigner"},
	DocGenInd:                              {"DocGenInd"},
	DocumentChecklist:                      {"DocumentChecklist"},
	FlowSites:                              {"FlowSites"},
	Fundraising:                            {"Fundraising"},
	IndustriesActionPlan:                   {"IndustriesActionPlan"},
	IndustriesSalesExcellenceAddOn:         {"IndustriesSalesExcellenceAddOn"},
	IndustriesServiceExcellenceAddOn:       {"IndustriesServiceExcellenceAddOn"},
	LightningScheduler:                     {"LightningScheduler"},
	LightningServiceConsole:                {"LightningServiceConsole"},
	MarketingUser:                          {"MarketingUser"},
	OmniStudioDesigner:                     {"OmniStudioDesigner"},
	OmniStudioRuntime:                      {"OmniStudioRuntime"},
	PublicSectorAccess:                     {"PublicSectorAccess"},
	SharedActivities:                       {"SharedActivities"},
	HighVelocitySales:                      {"HighVelocitySales"},
	InvoiceManagement:                      {"InvoiceManagement"},
	WorkplaceCommandCenterUser:             {"WorkplaceCommandCenterUser"},
	ForceComPlatform:                       {"ForceComPlatform"},
	Division:                               {"Division"},
	Grantmaking:                            {"Grantmaking"},
	OutcomeManagement:                      {"OutcomeManagement"},
	NonprofitCloudCaseManagementUser:       {"NonprofitCloudCaseManagementUser"},
	VolunteerManagement:                    {"VolunteerManagement"},
	FinancialServicesInsuranceUser:         {"FinancialServicesInsuranceUser"},
	FSCAlertFramework:                      {"FSCAlertFramework"},
	FSCServiceProcess:                      {"FSCServiceProcess"},
	IndustriesBranchManagement:             {"IndustriesBranchManagement"},
	IndustriesCompliantDataSharing:         {"IndustriesCompliantDataSharing"},
	InsuranceCalculationUser:               {"InsuranceCalculationUser"},
	InsuranceClaimMgmt:                     {"InsuranceClaimMgmt"},
	InsurancePolicyAdmin:                   {"InsurancePolicyAdmin"},
	BusinessRulesEngine:                    {"BusinessRulesEngine"},
	ApexIntegrationTests:                   {"ApexIntegrationTests"},
	IndustriesManufacturingCmty:            {"IndustriesManufacturingCmty"},
	IndustriesMfgAccountForecast:           {"IndustriesMfgAccountForecast"},
	IndustriesMfgAdvncdAccFrcs:             {"IndustriesMfgAdvncdAccFrcs"},
	IndustriesMfgPartnerVisitMgmt:          {"IndustriesMfgPartnerVisitMgmt"},
	IndustriesMfgProgram:                   {"IndustriesMfgProgram"},
	IndustriesMfgRebates:                   {"IndustriesMfgRebates"},
	IndustriesMfgTargets:                   {"IndustriesMfgTargets"},
	LifeSciencesCloudForCustomerEngagement: {"LifeSciencesCloudForCustomerEngagement"},
	EnablePRM:                              {"EnablePRM"},
}

type ScratchProduct enumflag.Flag

const (
	FSC ScratchProduct = iota
	CommunitiesProduct
	HealthCloudProduct
	CRMAnalyticsProduct
	B2BCommerceProduct
	LiveAgentProduct
	KnowledgeProduct
	RevenueCloudProduct
	MessagingProduct
	EducationCloudProduct
	WorkComProduct
	NonprofitCloudProduct
	ManufacturingCloudProduct
	LifeSciencesProduct
)

var ScratchProductIds = map[ScratchProduct][]string{
	FSC:                       {"fsc"},
	CommunitiesProduct:        {"communities"},
	HealthCloudProduct:        {"healthcloud"},
	CRMAnalyticsProduct:       {"crmanalytics"},
	B2BCommerceProduct:        {"b2bcommerce"},
	LiveAgentProduct:          {"liveagent"},
	KnowledgeProduct:          {"knowledge"},
	RevenueCloudProduct:       {"revenuecloud"},
	MessagingProduct:          {"messaging"},
	EducationCloudProduct:     {"educationcloud"},
	WorkComProduct:            {"work.com"},
	NonprofitCloudProduct:     {"nonprofitcloud"},
	ManufacturingCloudProduct: {"manufacturingcloud"},
	LifeSciencesProduct:       {"lifesciences"},
}

type ScratchEdition enumflag.Flag

const (
	Developer ScratchEdition = iota
	Enterprise
	Group
	Professional
	PartnerDeveloper
	PartnerEnterprise
	PartnerGroup
	PartnerProfessional
)

// The first value for each edition is the ScratchOrgInfo Edition picklist
// value sent to Salesforce; any additional values are accepted flag aliases.
var ScratchEditionIds = map[ScratchEdition][]string{
	Developer:           {"Developer"},
	Enterprise:          {"Enterprise"},
	Group:               {"Group"},
	Professional:        {"Professional"},
	PartnerDeveloper:    {"Partner Developer", "PartnerDeveloper"},
	PartnerEnterprise:   {"Partner Enterprise", "PartnerEnterprise"},
	PartnerGroup:        {"Partner Group", "PartnerGroup"},
	PartnerProfessional: {"Partner Professional", "PartnerProfessional"},
}

type ScratchSetting enumflag.Flag

const (
	EnableEnhancedNotes ScratchSetting = iota
	EnableQuote
	NetworksEnabled
	CommerceEnabled
	EnableApexApprovalLockUnlock
	PermsetsInFieldCreation
	EnableLightningPreviewPref
	EnableOrders
	EnableEnhancedCommerceOrders
	EnableLiveAgent
	EnableMultiCurrency
	EnableCoreCPQ
	EnableSubscriptionManagement
	EnableKnowledge
	EnableLightningKnowledge
	EnableBillingSetup
	EnableExperienceBundleMetadata
	EnableS1DesktopEnabled
	EnableContextDefinitions
	EnableEinsteinGptPlatform
	EnableOpportunityTeam
	EnableOrderManagementSetting
	EnableOrderEvents
	EnableOptionalPricebook
	EnableZeroQuantity
	EnableNegativeQuantity
	EnableQuotesWithoutOppEnabled
	EnableHighAvailability
	EnablePricingWaterfall
	EnablePricingWaterfallPersistence
	EnableSalesforcePricing
	EnableRating
	EnableRatingWaterfall
	EnableRatingWaterfallPersistence
	EnableProductConfigurator
	EnableDFOPref
	EnableRelateContactToMultipleAccounts
	EnableDisableParallelApexTesting
	EnableChatter
	EnableCommunityWorkspaces
	DeleteDCIWithFiles
	EnableForecasts
	EnableS1EncryptedStoragePref2
	EnableAcademicOperations
	EnableAlumniRelations
	EnableBenefitManagementPreference
	EnableBenefitAndGoalSharingPref
	EnableCarePlansPreference
	EnableDiscoveryFrameworkMetadata
	EnableEducationCloud
	EnableFundraising
	EnableGroupMembershipPref
	EnableIndustriesAssessment
	EnableInteractionSummaryPref
	EnableInteractionSummaryRoleHierarchy
	EnableStudentSuccess
	EnableInterestTagging
	EnableMiddleName
	EnableNameSuffix
	EnableRevenueSchedule
	EnableEnhancedPermsetMgmt
	EnableEnhancedProfileMgmt
	EnableNewProfileUI
	EnableSurvey
	EnableGrantmaking
	EnableOutcomes
	EnableCompliantDataSharingForBudget
	EnableCompliantDataSharingForIndividualApplication
	EnableCompliantDataSharingForFundingAward
	EnableDisbursementPreference
	EnableGiftAgreement
	EnableGiftEntryGrid
	EnableGiftPlanning
	EnableProgramCohorts
	EnableRecordRollup
	EnableVolunteerManagement
	EnableIndManufacturing
	EnableIndustriesMfgAccountForecast
	EnableIndustriesMfgAdvForecast
	EnableIndustriesMfgTargets
	EnableIndustriesMfgProgram
	EnablePtnrLeadMgmtMappings
	EnableInventoryAllocation
	EnableSampleManagement
	EnableInventoryReplenishment
	EnablePurchaseOrderMgt
	EnableWarrantyLCMgmt
	EnableMfgServiceConsole
	EnableIndustriesFieldService
	EnableAutomotiveServiceCampaignManagement
	EnableLifeSciencesCustomerEngagementBase
	EnableLifeSciencesActivityPlan
	EnableLifeSciencesClinialTrailManagement
	EnableLifeSciencesConsent
	EnableLifeSciencesMergeManagement
	EnableLifeSciencesProviderEngagementCompliance
	EnableLifeSciencesSiteManagement
	EnableLifeSciGenericWorkflow
	EnableLifeSciInquiry
	EnableLifeSciListsAndFilters
	EnableAdverseEvents
	EnableTrialManagementRandomization
	EnableSampleLimits
	EnableVisitInventoryEnabled
	EnableVisitPlanningAndExecutionAction
	EnablePATSTerritoryBasedSharing
	EnableBatchManagement
	EnableInventoryCount
	EnableProductInventoryOperations
	EnableTerritoryManagement2
	EnableLSC4CEVisits
	EnableActionableList
	EnableAppAlerts
	EnableNextBestAction
	EnableNextBestCustomer
	EnableNextBestMessage
	EnableAccountPlan
)

var ScratchSettingIds = map[ScratchSetting][]string{
	EnableEnhancedNotes:                   {"enableEnhancedNotes"},
	EnableQuote:                           {"enableQuote"},
	NetworksEnabled:                       {"networksEnabled"},
	CommerceEnabled:                       {"commerceEnabled"},
	EnableApexApprovalLockUnlock:          {"enableApexApprovalLockUnlock"},
	PermsetsInFieldCreation:               {"permsetsInFieldCreation"},
	EnableLightningPreviewPref:            {"enableLightningPreviewPref"},
	EnableOrders:                          {"enableOrders"},
	EnableEnhancedCommerceOrders:          {"enableEnhancedCommerceOrders"},
	EnableLiveAgent:                       {"enableLiveAgent"},
	EnableMultiCurrency:                   {"enableMultiCurrency"},
	EnableCoreCPQ:                         {"enableCoreCPQ"},
	EnableSubscriptionManagement:          {"enableSubscriptionManagement"},
	EnableKnowledge:                       {"enableKnowledge"},
	EnableLightningKnowledge:              {"enableLightningKnowledge"},
	EnableBillingSetup:                    {"enableBillingSetup"},
	EnableExperienceBundleMetadata:        {"enableExperienceBundleMetadata"},
	EnableS1DesktopEnabled:                {"enableS1DesktopEnabled"},
	EnableContextDefinitions:              {"enableContextDefinitions"},
	EnableEinsteinGptPlatform:             {"enableEinsteinGptPlatform"},
	EnableOpportunityTeam:                 {"enableOpportunityTeam"},
	EnableOrderManagementSetting:          {"enableOrderManagement"},
	EnableOrderEvents:                     {"enableOrderEvents"},
	EnableOptionalPricebook:               {"enableOptionalPricebook"},
	EnableZeroQuantity:                    {"enableZeroQuantity"},
	EnableNegativeQuantity:                {"enableNegativeQuantity"},
	EnableQuotesWithoutOppEnabled:         {"enableQuotesWithoutOppEnabled"},
	EnableHighAvailability:                {"enableHighAvailability"},
	EnablePricingWaterfall:                {"enablePricingWaterfall"},
	EnablePricingWaterfallPersistence:     {"enablePricingWaterfallPersistence"},
	EnableSalesforcePricing:               {"enableSalesforcePricing"},
	EnableRating:                          {"enableRating"},
	EnableRatingWaterfall:                 {"enableRatingWaterfall"},
	EnableRatingWaterfallPersistence:      {"enableRatingWaterfallPersistence"},
	EnableProductConfigurator:             {"enableProductConfigurator"},
	EnableDFOPref:                         {"enableDFOPref"},
	EnableRelateContactToMultipleAccounts: {"enableRelateContactToMultipleAccounts"},
	EnableDisableParallelApexTesting:      {"enableDisableParallelApexTesting"},
	EnableChatter:                         {"enableChatter"},
	EnableCommunityWorkspaces:             {"enableCommunityWorkspaces"},
	DeleteDCIWithFiles:                    {"deleteDCIWithFiles"},
	EnableForecasts:                       {"enableForecasts"},
	EnableS1EncryptedStoragePref2:         {"enableS1EncryptedStoragePref2"},
	EnableAcademicOperations:              {"enableAcademicOperations"},
	EnableAlumniRelations:                 {"enableAlumniRelations"},
	EnableBenefitManagementPreference:     {"enableBenefitManagementPreference"},
	EnableBenefitAndGoalSharingPref:       {"enableBenefitAndGoalSharingPref"},
	EnableCarePlansPreference:             {"enableCarePlansPreference"},
	EnableDiscoveryFrameworkMetadata:      {"enableDiscoveryFrameworkMetadata"},
	EnableEducationCloud:                  {"enableEducationCloud"},
	EnableFundraising:                     {"enableFundraising"},
	EnableGroupMembershipPref:             {"enableGroupMembershipPref"},
	EnableIndustriesAssessment:            {"enableIndustriesAssessment"},
	EnableInteractionSummaryPref:          {"enableInteractionSummaryPref"},
	EnableInteractionSummaryRoleHierarchy: {"enableInteractionSummaryRoleHierarchy"},
	EnableStudentSuccess:                  {"enableStudentSuccess"},
	EnableInterestTagging:                 {"enableInterestTagging"},
	EnableMiddleName:                      {"enableMiddleName"},
	EnableNameSuffix:                      {"enableNameSuffix"},
	EnableRevenueSchedule:                 {"enableRevenueSchedule"},
	EnableEnhancedPermsetMgmt:             {"enableEnhancedPermsetMgmt"},
	EnableEnhancedProfileMgmt:             {"enableEnhancedProfileMgmt"},
	EnableNewProfileUI:                    {"enableNewProfileUI"},
	EnableSurvey:                          {"enableSurvey"},

	EnableGrantmaking:                                  {"enableGrantmaking"},
	EnableOutcomes:                                     {"enableOutcomes"},
	EnableCompliantDataSharingForBudget:                {"enableCompliantDataSharingForBudget"},
	EnableCompliantDataSharingForIndividualApplication: {"enableCompliantDataSharingForIndividualApplication"},
	EnableCompliantDataSharingForFundingAward:          {"enableCompliantDataSharingForFundingAward"},
	EnableDisbursementPreference:                       {"enableDisbursementPreference"},
	EnableGiftAgreement:                                {"enableGiftAgreement"},
	EnableGiftEntryGrid:                                {"enableGiftEntryGrid"},
	EnableGiftPlanning:                                 {"enableGiftPlanning"},
	EnableProgramCohorts:                               {"enableProgramCohorts"},
	EnableRecordRollup:                                 {"enableRecordRollup"},
	EnableVolunteerManagement:                          {"enableVolunteerManagement"},

	EnableIndManufacturing:                    {"enableIndManufacturing"},
	EnableIndustriesMfgAccountForecast:        {"enableIndustriesMfgAccountForecast"},
	EnableIndustriesMfgAdvForecast:            {"enableIndustriesMfgAdvForecast"},
	EnableIndustriesMfgTargets:                {"enableIndustriesMfgTargets"},
	EnableIndustriesMfgProgram:                {"enableIndustriesMfgProgram"},
	EnablePtnrLeadMgmtMappings:                {"enablePtnrLeadMgmtMappings"},
	EnableInventoryAllocation:                 {"enableInventoryAllocation"},
	EnableSampleManagement:                    {"enableSampleManagement"},
	EnableInventoryReplenishment:              {"enableInventoryReplenishment"},
	EnablePurchaseOrderMgt:                    {"enablePurchaseOrderMgt"},
	EnableWarrantyLCMgmt:                      {"enableWarrantyLCMgmt"},
	EnableMfgServiceConsole:                   {"enableMfgServiceConsole"},
	EnableIndustriesFieldService:              {"enableIndustriesFieldService"},
	EnableAutomotiveServiceCampaignManagement: {"enableAutomotiveServiceCampaignManagement"},

	EnableLifeSciencesCustomerEngagementBase:       {"enableLifeSciencesCustomerEngagementBase"},
	EnableLifeSciencesActivityPlan:                 {"enableLifeSciencesActivityPlan"},
	EnableLifeSciencesClinialTrailManagement:       {"enableLifeSciencesClinialTrailManagement"},
	EnableLifeSciencesConsent:                      {"enableLifeSciencesConsent"},
	EnableLifeSciencesMergeManagement:              {"enableLifeSciencesMergeManagement"},
	EnableLifeSciencesProviderEngagementCompliance: {"enableLifeSciencesProviderEngagementCompliance"},
	EnableLifeSciencesSiteManagement:               {"enableLifeSciencesSiteManagement"},
	EnableLifeSciGenericWorkflow:                   {"enableLifeSciGenericWorkflow"},
	EnableLifeSciInquiry:                           {"enableLifeSciInquiry"},
	EnableLifeSciListsAndFilters:                   {"enableLifeSciListsAndFilters"},
	EnableAdverseEvents:                            {"enableAdverseEvents"},
	EnableTrialManagementRandomization:             {"enableTrialManagementRandomization"},
	EnableSampleLimits:                             {"enableSampleLimits"},
	EnableVisitInventoryEnabled:                    {"enableVisitInventoryEnabled"},
	EnableVisitPlanningAndExecutionAction:          {"enableVisitPlanningAndExecutionAction"},
	EnablePATSTerritoryBasedSharing:                {"enablePATSTerritoryBasedSharing"},
	EnableBatchManagement:                          {"enableBatchManagement"},
	EnableInventoryCount:                           {"enableInventoryCount"},
	EnableProductInventoryOperations:               {"enableProductInventoryOperations"},
	EnableTerritoryManagement2:                     {"enableTerritoryManagement2"},
	EnableLSC4CEVisits:                             {"enableLSC4CEVisits"},
	EnableActionableList:                           {"enableActionableList"},
	EnableAppAlerts:                                {"enableAppAlerts"},
	EnableNextBestAction:                           {"enableNextBestAction"},
	EnableNextBestCustomer:                         {"enableNextBestCustomer"},
	EnableNextBestMessage:                          {"enableNextBestMessage"},
	EnableAccountPlan:                              {"enableAccountPlan"},
}

type ScratchRelease enumflag.Flag

const (
	ReleaseDefault ScratchRelease = iota
	ReleasePreview
	ReleasePrevious
)

var ScratchReleaseIds = map[ScratchRelease][]string{
	ReleaseDefault:  {""},
	ReleasePreview:  {"preview"},
	ReleasePrevious: {"previous"},
}

var (
	selectedFeatures  []ScratchFeature
	selectedProducts  []ScratchProduct
	selectedEdition   ScratchEdition
	selectedSettings  []ScratchSetting
	selectedRelease   ScratchRelease
	featureQuantities map[string]string
)

var featuresRequiringQuantity = map[string]bool{
	"FinancialServicesUser":             true,
	"ScvMultipartyAndConsult":           true,
	"ServiceCloudVoicePartnerTelephony": true,
	"BYOOTT":                            true,
	"EducationCloud":                    true,
	"InsurancePolicyAdmin":              true,
	"InsuranceClaimMgmt":                true,
}

const defaultFeatureQuantity = "10"

func init() {
	loginCmd.Flags().StringP("user", "u", "", "username for SOAP or OAuth Username-Password login")
	loginCmd.Flags().StringP("password", "p", "", "password for SOAP or OAuth Username-Password login")
	loginCmd.Flags().StringP("api-version", "v", "", "API version to use")
	loginCmd.Flags().String("connected-app-client-id", "", "Client Id (aka Consumer Key) to use instead of default")
	loginCmd.Flags().StringP("key", "k", "", "JWT signing key filename")
	loginCmd.Flags().String("connected-app-client-secret", "", "Client Secret (aka Consumer Secret) for Client Credentials flow")
	loginCmd.Flags().BoolP("skip", "s", false, "skip login if already authenticated and only save token (useful with SSO)")
	loginCmd.Flags().StringP("instance", "i", "", `Defaults to 'login' or last
logged in system. non-production server to login to (values are 'pre',
'test', or full instance url`)
	loginCmd.Flags().IntP("port", "P", DefaultOAuthCallbackPort, "port for local OAuth callback server")
	loginCmd.Flags().Bool("device-flow", false, "use OAuth Device Flow (for headless environments); requires --connected-app-client-id")

	scratchCmd.Flags().String("username", "", "username for scratch org user")
	scratchCmd.Flags().Var(
		enumflag.NewSlice(&selectedFeatures, "feature", ScratchFeatureIds, enumflag.EnumCaseInsensitive),
		"feature",
		"feature to enable (can be specified multiple times); see command help for available features")
	scratchCmd.Flags().Var(
		enumflag.NewSlice(&selectedProducts, "product", ScratchProductIds, enumflag.EnumCaseInsensitive),
		"product",
		"product shortcut for features (can be specified multiple times); see command help for available products")
	scratchCmd.Flags().StringToString("quantity", map[string]string{}, "override default quantity for features (e.g., FinancialServicesUser=5); default quantity is 10")
	scratchCmd.Flags().String("namespace", "", "namespace for the scratch org")
	scratchCmd.Flags().Var(
		enumflag.New(&selectedEdition, "edition", ScratchEditionIds, enumflag.EnumCaseInsensitive),
		"edition",
		"scratch org edition; see command help for available editions")
	scratchCmd.Flags().Var(
		enumflag.NewSlice(&selectedSettings, "setting", ScratchSettingIds, enumflag.EnumCaseInsensitive),
		"setting",
		"setting to enable (can be specified multiple times); see command help for available settings")
	scratchCmd.Flags().Var(
		enumflag.New(&selectedRelease, "release", ScratchReleaseIds, enumflag.EnumCaseInsensitive),
		"release",
		"Salesforce release for scratch org: preview (next release) or previous")
	scratchCmd.Flags().Int("duration", 7, "number of days before the scratch org expires (1-30)")

	loginCmd.AddCommand(scratchCmd)
	RootCmd.AddCommand(loginCmd)
}

var scratchCmd = &cobra.Command{
	Use:   "scratch",
	Short: "Create scratch org and log in",
	Long: `Create scratch org and log in

Available Features:
  AccountingSubledgerGrowthEdition    - Enables Accounting Subledger Growth Edition
  AccountingSubledgerUser             - Enables Accounting Subledger user licenses
  AdmissionsConnectUser               - Enables Admissions Connect user licenses
  AdvisorLinkFeature                  - Enables Advisor Link
  AdvisorLinkPathwaysFeature          - Enables Advisor Link Pathways
  AnalyticsAdminPerms                 - Enables CRM Analytics admin permissions
  AnalyticsQueryService               - Enables Analytics Query Service
  ApexIntegrationTests                - Enables Apex integration tests (@IntegrationTest classes)
  ApexUserModeWithPermset             - Enables Apex code to run in user mode with a permission set session
  Assessments                         - Enables Assessments
  B2BCommerce                         - Enables B2B Commerce
  BillingAdvanced                     - Enables Advanced Billing (Revenue Cloud)
  BYOOTT                              - Enables Bring Your Own Over-The-Top messaging channel (requires quantity, default: 10)
  Communities                         - Enables Experience Cloud (Communities)
  ContactsToMultipleAccounts          - Allows a single Contact to be associated with multiple Accounts
  CoreCpq                             - Enables Revenue Cloud / Salesforce CPQ Core
  CustomerCommunityPlus               - Enables Customer Community Plus user licenses
  DataProcessingEngine                - Enables Data Processing Engine
  DecisionTable                       - Enables Decision Table
  DevelopmentWave                     - Enables CRM Analytics development features
  DocGen                              - Enables Document Generation
  DocGenDesigner                      - Enables Document Generation Designer
  DocGenInd                           - Enables Industries Document Generation
  Division                            - Enables Divisions
  DocumentChecklist                   - Enables Document Checklist
  DSARPortability                     - Enables Data Subject Access Request (DSAR) data portability
  EducationCloud                      - Enables Education Cloud user licenses (requires quantity, default: 10)
  Einstein1AIPlatform                 - Enables Einstein 1 AI Platform
  EinsteinAnalyticsPlus               - Enables Einstein Analytics Plus
  EinsteinBuilderFree                 - Enables Einstein Builder Free
  EmbeddedServiceMessaging            - Enables Embedded Service Messaging (Messaging for In-App and Web)
  Enablement                          - Enables Enablement (in-app guided learning programs)
  EnableSetPasswordInApi              - Allows setting passwords via API
  EventLogFile                        - Enables Event Log File
  FinancialServicesUser               - Enables Financial Services Cloud user licenses (requires quantity, default: 10)
  FlowSites                           - Enables Flow Sites
  ForceComPlatform                    - Enables Force.com Platform user licenses (always included in base features)
  Fundraising                         - Enables Fundraising
  Grantmaking                         - Enables Grantmaking (Nonprofit Cloud)
  HealthCloudAddOn                    - Enables Health Cloud add-on
  HealthCloudUser                     - Enables Health Cloud user licenses
  HighVelocitySales                   - Enables High Velocity Sales (Sales Engagement)
  IndustriesActionPlan                - Enables Industries Action Plans
  IndustriesSalesExcellenceAddOn      - Enables Industries Sales Excellence Add-On
  IndustriesServiceExcellenceAddOn    - Enables Industries Service Excellence Add-On
  IndustriesManufacturingCmty         - Enables the Manufacturing Sales Agreement for the Community permission set license and the Manufacturing community template
  IndustriesMfgAccountForecast        - Enables Account Forecasts (Manufacturing Cloud)
  IndustriesMfgAdvncdAccFrcs          - Enables Advanced Account Forecasting (Manufacturing Cloud; documented, but rejected by a Dev Hub as not a valid Features value in October 2026, so the manufacturingcloud product leaves it out)
  IndustriesMfgPartnerVisitMgmt       - Enables Partner Visit Management (Manufacturing Cloud)
  IndustriesMfgProgram                - Enables Program Based Business (Manufacturing Cloud)
  IndustriesMfgRebates                - Enables Rebate Management (Manufacturing Cloud)
  IndustriesMfgTargets                - Enables Sales Agreements (Manufacturing Cloud)
  EnablePRM                           - Enables the partner relationship management permissions for the org
  InsightsPlatform                    - Enables Insights Platform
  LifeSciencesCloudForCustomerEngagement - Enables Life Sciences Cloud for Customer Engagement
  InvoiceManagement                   - Enables Invoice Management (Revenue Cloud)
  Knowledge                           - Enables Salesforce Knowledge
  LightningScheduler                  - Enables Lightning Scheduler
  LightningServiceConsole             - Enables Lightning Service Console
  LiveAgent                           - Enables Live Agent (Chat)
  LiveMessage                         - Enables LiveMessage (SMS/MMS messaging)
  MarketingUser                       - Enables Marketing User licenses
  NonprofitCloudCaseManagementUser    - Enables Nonprofit Cloud Case Management user licenses
  OmniStudioDesigner                  - Enables OmniStudio Designer
  OmniStudioRuntime                   - Enables OmniStudio Runtime
  OrderManagement                     - Enables Salesforce Order Management
  OrderSaveLogicEnabled               - Enables order save behavior logic
  OutcomeManagement                   - Enables Outcome Management
  PartnerCommunity                    - Enables Partner Community user licenses
  PersonAccounts                      - Enables Person Accounts (B2C account model)
  PlatformCache                       - Enables Platform Cache
  PlatformEncryption                  - Enables Shield Platform Encryption
  ProgramManagement                   - Enables Program Management Module (Salesforce.org Nonprofit/Education)
  PublicSectorAccess                  - Enables Public Sector Access
  RevSubscriptionManagement           - Enables Subscription Management (B2B subscriptions and one-time sales)
  ScvMultipartyAndConsult             - Enables Service Cloud Voice multiparty and consult (requires quantity, default: 10)
  ServiceCloud                        - Enables Service Cloud
  ServiceCloudVoicePartnerTelephony   - Enables Service Cloud Voice Partner Telephony (requires quantity 1-50, default: 10)
  SharedActivities                    - Enables Shared Activities
  Slack                               - Enables Salesforce-Slack integration (required for SlackApp metadata and slash command registration)
  StateAndCountryPicklist             - Enables State and Country Picklists for standard address fields
  SurveyAdvancedFeatures              - Enables advanced Salesforce Surveys features
  UsageManagement                     - Enables Usage Management (Revenue Cloud)
  VolunteerManagement                 - Enables Volunteer Management (Nonprofit Cloud)
  FinancialServicesInsuranceUser      - Enables Financial Services insurance components and objects
  FSCAlertFramework                   - Enables Financial Services Cloud Record Alert objects
  FSCServiceProcess                   - Enables Service Process Studio for Financial Services Cloud
  IndustriesBranchManagement          - Enables Branch Management (Financial Services Cloud)
  IndustriesCompliantDataSharing      - Enables participant management and Compliant Data Sharing
  InsuranceCalculationUser            - Enables Insurance calculation (Business Rules Engine and OmniStudio runtime licenses)
  InsuranceClaimMgmt                  - Enables Insurance claim management (requires quantity, default: 10)
  InsurancePolicyAdmin                - Enables Insurance policy administration (requires quantity, default: 10)
  BusinessRulesEngine                 - Enables Business Rules Engine (expression sets and lookup tables)
  WavePlatform                        - Enables Wave Platform (CRM Analytics)
  WorkplaceCommandCenterUser          - Enables Workplace Command Center user licenses (Work.com)

Available Products:
  b2bcommerce      - B2B Commerce (enables B2BCommerce, OrderManagement features and commerceEnabled, enableOrders, enableEnhancedCommerceOrders settings)
  communities      - Experience Cloud (enables Communities feature and networksEnabled setting)
  crmanalytics     - CRM Analytics (enables AnalyticsAdminPerms, WavePlatform, InsightsPlatform, EinsteinAnalyticsPlus, EinsteinBuilderFree, DevelopmentWave)
  educationcloud   - Education Cloud (enables EducationCloud, PersonAccounts, Communities, Knowledge, and many other Education Cloud features with a comprehensive set of industries/education settings)
  fsc              - Financial Services Cloud (enables PersonAccounts, ContactsToMultipleAccounts, FinancialServicesUser, the insurance policy, claim and calculation features, Record Alerts, Service Process, Branch Management, Compliant Data Sharing, Business Rules Engine, Data Processing Engine, Document Checklist, Action Plans, Assessments, OmniStudio and the Industries sales and service excellence add-ons, with the Interaction Summary, Record Rollup and Assessment industries settings)
  healthcloud      - Health Cloud (enables HealthCloudAddOn, HealthCloudUser)
  knowledge        - Salesforce Knowledge (enables Knowledge feature and enableKnowledge, enableLightningKnowledge settings)
  liveagent        - Live Agent (enables LiveAgent feature and enableLiveAgent setting)
  lifesciences     - Life Sciences Cloud (enables LifeSciencesCloudForCustomerEngagement, HealthCloudAddOn, HealthCloudUser, PersonAccounts, DocumentChecklist, Action Plans and the Industries sales excellence add-on, with the Life Sciences industries settings, Unified Inventory, Account Plans, Surveys, Industries Assessment and Sales Territories; Clinical Trial Management, Research Study Randomization and Visit Planning are left out because a scratch org refuses them)
  messaging        - Messaging (enables EmbeddedServiceMessaging, LiveMessage, BYOOTT features)
  manufacturingcloud - Manufacturing Cloud (enables IndustriesMfgTargets, IndustriesMfgAccountForecast, IndustriesMfgProgram, IndustriesMfgPartnerVisitMgmt, IndustriesMfgRebates, IndustriesManufacturingCmty, Communities, PartnerCommunity, OrderManagement, ServiceCloud, Data Processing Engine, Decision Table, OmniStudio, Action Plans and the Industries sales and service excellence add-ons, with the Sales Agreements, Account Forecasts, Advanced Account Forecasting, Account Manager Targets and Manufacturing Programs settings)
  nonprofitcloud   - Nonprofit Cloud (enables Fundraising, Grantmaking, OutcomeManagement, ProgramManagement, VolunteerManagement, NonprofitCloudCaseManagementUser, FinancialServicesUser (for Record Rollups), PersonAccounts, OmniStudio, and other Nonprofit Cloud features with the Nonprofit Cloud industries settings)
  revenuecloud     - Revenue Cloud (enables CoreCpq, BillingAdvanced, UsageManagement, DocGen, Einstein1AIPlatform, InvoiceManagement, OrderManagement, Communities, PartnerCommunity, CustomerCommunityPlus, EnableSetPasswordInApi, OrderSaveLogicEnabled features and a comprehensive set of billing/order/quote/pricing/rating settings)
  work.com         - Work.com (enables Communities, WorkplaceCommandCenterUser, ForceComPlatform features and enableS1DesktopEnabled, enableSurvey, networksEnabled settings)

Available Editions:
  Developer           - Developer Edition (default)
  Enterprise          - Enterprise Edition
  Group               - Group Edition
  Professional        - Professional Edition
  PartnerDeveloper    - Partner Developer Edition
  PartnerEnterprise   - Partner Enterprise Edition
  PartnerGroup        - Partner Group Edition
  PartnerProfessional - Partner Professional Edition

Available Settings (deployed after org creation):
  enableEnhancedNotes               - Enable Enhanced Notes
  enableQuote                       - Enable Quotes
  enableQuotesWithoutOppEnabled     - Allow quotes without Opportunity (QuoteSettings)
  networksEnabled                   - Enable Experience Cloud (Communities)
  commerceEnabled                   - Enable Commerce
  enableApexApprovalLockUnlock      - Allow Apex to lock/unlock approval processes
  permsetsInFieldCreation           - Allow assigning permission sets during field creation
  enableLightningPreviewPref        - Enable Lightning Experience preview pref
  enableS1DesktopEnabled            - Enable Lightning Experience on desktop (LightningExperienceSettings)
  enableOrders                      - Enable Orders
  enableEnhancedCommerceOrders      - Enable Enhanced Commerce Orders
  enableOrderEvents                 - Enable Order Events (OrderSettings)
  enableOptionalPricebook           - Make Pricebook optional on orders (OrderSettings)
  enableZeroQuantity                - Allow zero-quantity order items (OrderSettings)
  enableNegativeQuantity            - Allow negative-quantity order items (OrderSettings)
  enableOrderManagement             - Enable Order Management (OrderManagementSettings)
  enableLiveAgent                   - Enable Live Agent (Chat)
  enableMultiCurrency               - Enable Multi-Currency
  enableCoreCPQ                     - Enable Revenue Cloud / Salesforce CPQ Core (RevenueManagementSettings)
  enableSubscriptionManagement      - Enable Subscription Management (SubscriptionManagementSettings)
  enableKnowledge                   - Enable Salesforce Knowledge (KnowledgeSettings)
  enableLightningKnowledge          - Enable Lightning Knowledge (KnowledgeSettings)
  enableBillingSetup                - Enable Billing setup (BillingSettings)
  enableExperienceBundleMetadata    - Enable Experience Bundle metadata (ExperienceBundleSettings)
  enableContextDefinitions          - Enable Industries Context Definitions (IndustriesContextSettings)
  enableEinsteinGptPlatform         - Enable Einstein GPT Platform (EinsteinGptSettings)
  enableOpportunityTeam             - Enable Opportunity Teams (OpportunitySettings)
  enableHighAvailability            - Enable Pricing high availability (IndustriesPricingSettings)
  enablePricingWaterfall            - Enable Pricing Waterfall (IndustriesPricingSettings)
  enablePricingWaterfallPersistence - Persist Pricing Waterfall (IndustriesPricingSettings)
  enableSalesforcePricing           - Enable Salesforce Pricing (IndustriesPricingSettings)
  enableRating                      - Enable Rating engine (IndustriesRatingSettings)
  enableRatingWaterfall             - Enable Rating Waterfall (IndustriesRatingSettings)
  enableRatingWaterfallPersistence  - Persist Rating Waterfall (IndustriesRatingSettings)
  enableProductConfigurator         - Enable Product Configurator (ProductConfiguratorSettings)
  enableDFOPref                     - Enable Dynamic Fulfillment Orchestrator (DynamicFulfillmentOrchestratorSettings)
  enableRelateContactToMultipleAccounts - Allow a Contact to be related to multiple Accounts (AccountSettings)
  enableDisableParallelApexTesting      - Disable parallel Apex testing (ApexSettings)
  enableChatter                         - Enable Chatter (ChatterSettings)
  enableCommunityWorkspaces             - Enable Community Workspaces (CommunitiesSettings)
  deleteDCIWithFiles                    - Delete Document Checklist Items with files (DocumentChecklistSettings)
  enableForecasts                       - Enable Forecasts (ForecastingSettings)
  enableS1EncryptedStoragePref2         - Disable S1 encrypted storage (MobileSettings)
  enableAcademicOperations              - Enable Academic Operations (IndustriesSettings)
  enableAlumniRelations                 - Enable Alumni Relations (IndustriesSettings)
  enableBenefitManagementPreference     - Enable Benefit Management (IndustriesSettings)
  enableBenefitAndGoalSharingPref       - Enable Benefit and Goal Sharing (IndustriesSettings)
  enableCarePlansPreference             - Enable Care Plans (IndustriesSettings)
  enableCompliantDataSharingForBudget   - Enable Compliant Data Sharing for Budget (IndustriesSettings)
  enableCompliantDataSharingForFundingAward - Enable Compliant Data Sharing for Funding Award (IndustriesSettings)
  enableCompliantDataSharingForIndividualApplication - Enable Compliant Data Sharing for Individual Application (IndustriesSettings)
  enableDisbursementPreference          - Enable Benefit Disbursements (IndustriesSettings)
  enableDiscoveryFrameworkMetadata      - Enable Discovery Framework Metadata (IndustriesSettings)
  enableEducationCloud                  - Enable Education Cloud (IndustriesSettings)
  enableFundraising                     - Enable Fundraising (IndustriesSettings)
  enableGiftAgreement                   - Enable Gift Agreements (IndustriesSettings)
  enableGiftEntryGrid                   - Enable Gift Entry Grid (IndustriesSettings)
  enableGiftPlanning                    - Enable Gift Planning (IndustriesSettings)
  enableGrantmaking                     - Enable Grantmaking (IndustriesSettings)
  enableGroupMembershipPref             - Enable Group Membership (IndustriesSettings)
  enableIndustriesAssessment            - Enable Industries Assessment (IndustriesSettings)
  enableInteractionSummaryPref          - Enable Interaction Summary (IndustriesSettings)
  enableInteractionSummaryRoleHierarchy - Enable Interaction Summary Role Hierarchy (IndustriesSettings)
  enableOutcomes                        - Enable Outcome Management (IndustriesSettings)
  enableProgramCohorts                  - Enable Program Cohorts (IndustriesSettings)
  enableRecordRollup                    - Enable Record Rollups (IndustriesSettings)
  enableStudentSuccess                  - Enable Student Success (IndustriesSettings)
  enableVolunteerManagement             - Enable Volunteer Management (IndustriesSettings)
  enableIndManufacturing                - Enable Sales Agreements (IndustriesManufacturingSettings)
  enableIndustriesMfgAccountForecast    - Enable Account Forecasts (IndustriesManufacturingSettings)
  enableIndustriesMfgAdvForecast        - Enable Advanced Account Forecasting (IndustriesManufacturingSettings)
  enableIndustriesMfgProgram            - Enable Manufacturing Programs (IndustriesManufacturingSettings)
  enableIndustriesMfgTargets            - Enable Account Manager Targets (IndustriesManufacturingSettings)
  enablePtnrLeadMgmtMappings            - Enable Partner Lead Management mappings (IndustriesManufacturingSettings)
  enableInventoryAllocation             - Enable Inventory Allocation (InventoryAllocationSettings)
  enableSampleManagement                - Enable Sample Management (IndustriesMfgSampleManagementSettings)
  enableInventoryReplenishment          - Enable Inventory Replenishment (InventoryReplenishmentSettings)
  enablePurchaseOrderMgt                - Enable Purchase Order Management (PurchaseOrderMgmtSettings)
  enableWarrantyLCMgmt                  - Enable Warranty Lifecycle Management (WarrantyLifecycleMgmtSettings)
  enableMfgServiceConsole               - Enable Service Console for Manufacturing (MfgServiceConsoleSettings)
  enableIndustriesFieldService          - Enable Industries Field Service components (IndustriesFieldServiceSettings)
  enableAutomotiveServiceCampaignManagement - Enable Product Service Campaign Management components (IndustriesFieldServiceSettings)
  enableLifeSciencesCustomerEngagementBase - Enable the Life Sciences Customer Engagement base components (IndustriesSettings)
  enableLifeSciencesActivityPlan        - Enable Life Sciences activity plans (IndustriesSettings)
  enableLifeSciencesClinialTrailManagement - Enable Clinical Trial Management and its data model (the documented spelling) (IndustriesSettings)
  enableLifeSciencesConsent             - Enable Life Sciences consent (IndustriesSettings)
  enableLifeSciencesMergeManagement     - Enable Life Sciences merge management (IndustriesSettings)
  enableLifeSciencesProviderEngagementCompliance - Enable provider engagement compliance (IndustriesSettings)
  enableLifeSciencesSiteManagement      - Enable clinical trial site management (IndustriesSettings)
  enableLifeSciGenericWorkflow          - Enable the Life Sciences generic workflow engine (IndustriesSettings)
  enableLifeSciInquiry                  - Enable Life Sciences inquiries (IndustriesSettings)
  enableLifeSciListsAndFilters          - Enable Life Sciences lists and filters (IndustriesSettings)
  enableAdverseEvents                   - Enable Adverse Events and its data model (IndustriesSettings)
  enableTrialManagementRandomization    - Enable Research Study Randomization (IndustriesSettings)
  enableSampleLimits                    - Enable sample limits (IndustriesSettings)
  enableVisitInventoryEnabled           - Enable inventory tracking for visits (IndustriesSettings)
  enableVisitPlanningAndExecutionAction - Enable visit planning and execution actions (IndustriesSettings)
  enablePATSTerritoryBasedSharing       - Enable territory-based sharing for provider and affiliation tracking (IndustriesSettings)
  enableBatchManagement                 - Enable batch-based inventory management (IndustriesUnifiedInventorySettings)
  enableInventoryCount                  - Enable Inventory Count (IndustriesUnifiedInventorySettings)
  enableProductInventoryOperations      - Enable Product Inventory Operations (IndustriesUnifiedInventorySettings)
  enableTerritoryManagement2            - Enable Sales Territories (Enterprise Territory Management) (Territory2Settings)
  enableLSC4CEVisits                    - Enable Life Sciences Customer Engagement visits (ProviderVisit and the sample limit objects) (IndustriesSettings)
  enableActionableList                  - Enable actionable lists (IndustriesSettings)
  enableAppAlerts                       - Enable app alerts (IndustriesSettings)
  enableNextBestAction                  - Enable Next Best Action (TerritoryAcctRcmdAction) (IndustriesSettings)
  enableNextBestCustomer                - Enable Next Best Customer (TerritoryAccountScore) (IndustriesSettings)
  enableNextBestMessage                 - Enable Next Best Message (TerritoryAcctProdMsgScore) (IndustriesSettings)
  enableAccountPlan                     - Enable Account Plans (AccountPlanSettings)
  enableInterestTagging                 - Enable Interest Tagging (InterestTaggingSettings)
  enableMiddleName                      - Enable Middle Name (NameSettings)
  enableNameSuffix                      - Enable Name Suffix (NameSettings)
  enableRevenueSchedule                 - Enable Revenue Schedule (ProductSettings)
  enableSurvey                          - Enable Salesforce Surveys (SurveySettings)
  enableEnhancedPermsetMgmt             - Enable Enhanced Permission Set Management (UserManagementSettings)
  enableEnhancedProfileMgmt             - Enable Enhanced Profile Management (UserManagementSettings)
  enableNewProfileUI                    - Enable New Profile UI (UserManagementSettings)

Available Releases:
  preview  - Create scratch org on the next (preview) release
  previous - Create scratch org on the previous release

Examples:
  force login scratch --product fsc
  force login scratch --feature PersonAccounts --feature StateAndCountryPicklist
  force login scratch --product fsc --quantity FinancialServicesUser=20
  force login scratch --namespace myns
  force login scratch --edition Enterprise --product fsc
  force login scratch --setting enableEnhancedNotes
  force login scratch --setting enableQuote
  force login scratch --product b2bcommerce
  force login scratch --product communities
  force login scratch --product crmanalytics
  force login scratch --product educationcloud
  force login scratch --product healthcloud
  force login scratch --product knowledge
  force login scratch --product liveagent
  force login scratch --product messaging
  force login scratch --product nonprofitcloud
  force login scratch --product lifesciences
  force login scratch --product manufacturingcloud
  force login scratch --product revenuecloud
  force login scratch --product work.com
  force login scratch --release preview
  force login scratch --release previous
  force login scratch --duration 14`,
	Run: func(cmd *cobra.Command, args []string) {
		scratchUser, _ := cmd.Flags().GetString("username")
		scratchNamespace, _ := cmd.Flags().GetString("namespace")
		quantities, _ := cmd.Flags().GetStringToString("quantity")
		allFeatures := expandProductsToFeatures(selectedProducts, selectedFeatures, quantities)
		edition := ScratchEditionIds[selectedEdition][0]
		allSettings := expandProductsToSettings(selectedProducts, selectedSettings)
		release := ScratchReleaseIds[selectedRelease][0]
		duration, _ := cmd.Flags().GetInt("duration")
		scratchLogin(scratchUser, allFeatures, edition, allSettings, scratchNamespace, release, duration)
	},
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log into Salesforce and store a session token",
	Long: `Log into Salesforce and store a session token.  By default, OAuth is
used and a refresh token will be stored as well.  The refresh token is used
to get a new session token automatically when needed.`,
	Example: `
    force login
    force login -i test
    force login -i example--dev.sandbox.my.salesforce.com
    force login -u user@example.com -p password
    force login -i test -u user@example.com -p password
    force login -i my-domain.my.salesforce.com -u username -p password
    force login -i my-domain.my.salesforce.com -s[kipLogin]
    force login --connected-app-client-id <my-consumer-key> -u user@example.com -key jwt.key
    force login --connected-app-client-id <my-consumer-key> --connected-app-client-secret <my-consumer-secret>
    force login -P 8080
    force login --device-flow --connected-app-client-id <my-consumer-key>
    force login scratch
`,
	Args: cobra.MaximumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		connectedAppClientId, _ := cmd.Flags().GetString("connected-app-client-id")
		if connectedAppClientId != "" {
			ClientId = connectedAppClientId
		}
		endpoint := getEndpoint(cmd)
		selectApiVersion(cmd)
		username, _ := cmd.Flags().GetString("user")
		keyFile, _ := cmd.Flags().GetString("key")
		clientSecret, _ := cmd.Flags().GetString("connected-app-client-secret")
		switch {
		case username == "" && clientSecret != "":
			clientCredentialsLogin(endpoint, ClientId, clientSecret)
		case username == "":
			deviceFlow, _ := cmd.Flags().GetBool("device-flow")
			skipLogin, _ := cmd.Flags().GetBool("skip")
			port, _ := cmd.Flags().GetInt("port")
			if deviceFlow {
				if err := validateDeviceFlowClientId(connectedAppClientId); err != nil {
					ErrorAndExit(err.Error())
				}
				oauthDeviceFlowLogin(endpoint, skipLogin, port)
			} else {
				oauthLogin(endpoint, skipLogin, port)
			}
		case keyFile != "":
			jwtLogin(endpoint, username, keyFile)
		case clientSecret != "":
			password, _ := cmd.Flags().GetString("password")
			passwordLogin(endpoint, ClientId, clientSecret, username, password)
		default:
			password, _ := cmd.Flags().GetString("password")
			soapLogin(endpoint, username, password)
		}
	},
}

func expandProductsToFeatures(products []ScratchProduct, features []ScratchFeature, quantities map[string]string) []string {
	productFeatures := map[ScratchProduct][]ScratchFeature{
		FSC: {
			PersonAccounts,
			ContactsToMultipleAccounts,
			FinancialServicesUser,
			FinancialServicesInsuranceUser,
			InsurancePolicyAdmin,
			InsuranceClaimMgmt,
			InsuranceCalculationUser,
			FSCAlertFramework,
			FSCServiceProcess,
			IndustriesBranchManagement,
			IndustriesCompliantDataSharing,
			BusinessRulesEngine,
			DataProcessingEngine,
			DecisionTable,
			DocumentChecklist,
			IndustriesActionPlan,
			Assessments,
			OmniStudioRuntime,
			OmniStudioDesigner,
			IndustriesServiceExcellenceAddOn,
			IndustriesSalesExcellenceAddOn,
		},
		CommunitiesProduct:  {Communities},
		HealthCloudProduct:  {HealthCloudAddOn, HealthCloudUser},
		CRMAnalyticsProduct: {AnalyticsAdminPerms, WavePlatform, InsightsPlatform, EinsteinAnalyticsPlus, EinsteinBuilderFree, DevelopmentWave},
		B2BCommerceProduct:  {B2BCommerce, OrderManagement},
		LiveAgentProduct:    {LiveAgent},
		KnowledgeProduct:    {Knowledge},
		RevenueCloudProduct: {
			EnableSetPasswordInApi,
			OrderSaveLogicEnabled,
			OrderManagement,
			PartnerCommunity,
			Communities,
			CustomerCommunityPlus,
			CoreCpq,
			UsageManagement,
			BillingAdvanced,
			DocGen,
			Einstein1AIPlatform,
			InvoiceManagement,
		},
		MessagingProduct: {EmbeddedServiceMessaging, LiveMessage, BYOOTT},
		WorkComProduct:   {Communities, WorkplaceCommandCenterUser, ForceComPlatform},
		NonprofitCloudProduct: {
			AccountingSubledgerGrowthEdition,
			AccountingSubledgerUser,
			AnalyticsQueryService,
			Assessments,
			Communities,
			ContactsToMultipleAccounts,
			DataProcessingEngine,
			DecisionTable,
			DocGen,
			DocGenDesigner,
			DocGenInd,
			DocumentChecklist,
			EnableSetPasswordInApi,
			FinancialServicesUser,
			Fundraising,
			Grantmaking,
			IndustriesActionPlan,
			IndustriesSalesExcellenceAddOn,
			IndustriesServiceExcellenceAddOn,
			MarketingUser,
			NonprofitCloudCaseManagementUser,
			OmniStudioDesigner,
			OmniStudioRuntime,
			OutcomeManagement,
			PersonAccounts,
			ProgramManagement,
			PublicSectorAccess,
			StateAndCountryPicklist,
			VolunteerManagement,
		},
		LifeSciencesProduct: {
			DocumentChecklist,
			HealthCloudAddOn,
			HealthCloudUser,
			IndustriesActionPlan,
			IndustriesSalesExcellenceAddOn,
			LifeSciencesCloudForCustomerEngagement,
			PersonAccounts,
		},
		ManufacturingCloudProduct: {
			Communities,
			DataProcessingEngine,
			DecisionTable,
			EnableSetPasswordInApi,
			IndustriesActionPlan,
			IndustriesManufacturingCmty,
			IndustriesMfgAccountForecast,
			IndustriesMfgPartnerVisitMgmt,
			IndustriesMfgProgram,
			IndustriesMfgRebates,
			IndustriesMfgTargets,
			IndustriesSalesExcellenceAddOn,
			IndustriesServiceExcellenceAddOn,
			OmniStudioDesigner,
			OmniStudioRuntime,
			OrderManagement,
			PartnerCommunity,
			ServiceCloud,
			StateAndCountryPicklist,
		},
		EducationCloudProduct: {
			EducationCloud,
			AccountingSubledgerGrowthEdition,
			AccountingSubledgerUser,
			AdmissionsConnectUser,
			AdvisorLinkFeature,
			AdvisorLinkPathwaysFeature,
			AnalyticsQueryService,
			Assessments,
			Communities,
			ContactsToMultipleAccounts,
			DataProcessingEngine,
			DecisionTable,
			DocGen,
			DocGenDesigner,
			DocGenInd,
			DocumentChecklist,
			EnableSetPasswordInApi,
			FlowSites,
			Fundraising,
			IndustriesActionPlan,
			IndustriesSalesExcellenceAddOn,
			IndustriesServiceExcellenceAddOn,
			Knowledge,
			LightningScheduler,
			LightningServiceConsole,
			MarketingUser,
			OmniStudioDesigner,
			OmniStudioRuntime,
			PersonAccounts,
			PublicSectorAccess,
			SharedActivities,
			StateAndCountryPicklist,
		},
	}

	featureSet := make(map[ScratchFeature]bool)

	for _, product := range products {
		if pf, ok := productFeatures[product]; ok {
			for _, f := range pf {
				featureSet[f] = true
			}
		}
	}

	for _, feature := range features {
		featureSet[feature] = true
	}

	uniqueFeatures := make([]string, 0, len(featureSet))
	for feature := range featureSet {
		featureName := ScratchFeatureIds[feature][0]
		if featuresRequiringQuantity[featureName] {
			quantity := defaultFeatureQuantity
			if q, ok := quantities[featureName]; ok {
				quantity = q
			}
			featureName = featureName + ":" + quantity
		}
		uniqueFeatures = append(uniqueFeatures, featureName)
	}

	return uniqueFeatures
}

func convertSettingsToStrings(settings []ScratchSetting) []string {
	result := make([]string, 0, len(settings))
	for _, setting := range settings {
		result = append(result, ScratchSettingIds[setting][0])
	}
	return result
}

func expandProductsToSettings(products []ScratchProduct, settings []ScratchSetting) []string {
	productSettings := map[ScratchProduct][]ScratchSetting{
		CommunitiesProduct: {NetworksEnabled},
		B2BCommerceProduct: {CommerceEnabled, EnableOrders, EnableEnhancedCommerceOrders},
		LiveAgentProduct:   {EnableLiveAgent},
		KnowledgeProduct:   {EnableKnowledge, EnableLightningKnowledge},
		FSC: {
			EnableS1DesktopEnabled,
			EnableChatter,
			EnableEnhancedNotes,
			EnableIndustriesAssessment,
			EnableInteractionSummaryPref,
			EnableRecordRollup,
		},
		WorkComProduct: {EnableS1DesktopEnabled, EnableSurvey, NetworksEnabled},
		LifeSciencesProduct: {
			EnableS1DesktopEnabled,
			EnableChatter,
			EnableDisableParallelApexTesting,
			EnableEnhancedNotes,
			EnableContextDefinitions,
			EnableQuote,
			EnableLifeSciencesCustomerEngagementBase,
			EnableLifeSciencesActivityPlan,
			EnableLifeSciencesConsent,
			EnableLifeSciencesMergeManagement,
			EnableLifeSciencesProviderEngagementCompliance,
			EnableLifeSciencesSiteManagement,
			EnableLifeSciGenericWorkflow,
			EnableLifeSciInquiry,
			EnableLifeSciListsAndFilters,
			EnableAdverseEvents,
			EnableSampleLimits,
			EnableVisitInventoryEnabled,
			EnablePATSTerritoryBasedSharing,
			EnableBatchManagement,
			EnableInventoryCount,
			EnableProductInventoryOperations,
			EnableTerritoryManagement2,
			EnableLSC4CEVisits,
			EnableActionableList,
			EnableAppAlerts,
			EnableNextBestAction,
			EnableNextBestCustomer,
			EnableNextBestMessage,
			EnableAccountPlan,
			EnableIndustriesAssessment,
			EnableSurvey,
		},
		ManufacturingCloudProduct: {
			EnableS1DesktopEnabled,
			EnableChatter,
			EnableDisableParallelApexTesting,
			EnableEnhancedNotes,
			NetworksEnabled,
			EnableContextDefinitions,
			EnableIndManufacturing,
			EnableIndustriesMfgAccountForecast,
			EnableIndustriesMfgAdvForecast,
			EnableIndustriesMfgTargets,
			EnableIndustriesMfgProgram,
		},
		NonprofitCloudProduct: {
			EnableS1DesktopEnabled,
			EnableS1EncryptedStoragePref2,
			EnableChatter,
			EnableDisableParallelApexTesting,
			EnableEnhancedNotes,
			EnableBenefitManagementPreference,
			EnableBenefitAndGoalSharingPref,
			EnableCarePlansPreference,
			EnableFundraising,
			EnableGrantmaking,
			EnableGroupMembershipPref,
			EnableOutcomes,
			EnableDisbursementPreference,
			EnableGiftAgreement,
			EnableGiftEntryGrid,
			EnableGiftPlanning,
			EnableInteractionSummaryPref,
			EnableProgramCohorts,
			EnableRecordRollup,
			EnableVolunteerManagement,
			EnableMiddleName,
			EnableNameSuffix,
			EnableInterestTagging,
		},
		RevenueCloudProduct: {
			EnableBillingSetup,
			NetworksEnabled,
			EnableMultiCurrency,
			EnableExperienceBundleMetadata,
			EnableS1DesktopEnabled,
			EnableContextDefinitions,
			EnableEinsteinGptPlatform,
			EnableOpportunityTeam,
			EnableOrderManagementSetting,
			EnableOrders,
			EnableEnhancedCommerceOrders,
			EnableOrderEvents,
			EnableOptionalPricebook,
			EnableZeroQuantity,
			EnableNegativeQuantity,
			EnableQuote,
			EnableQuotesWithoutOppEnabled,
			EnableHighAvailability,
			EnablePricingWaterfall,
			EnablePricingWaterfallPersistence,
			EnableSalesforcePricing,
			EnableRating,
			EnableRatingWaterfall,
			EnableRatingWaterfallPersistence,
			EnableCoreCPQ,
			EnableProductConfigurator,
			EnableDFOPref,
		},
		EducationCloudProduct: {
			EnableRelateContactToMultipleAccounts,
			EnableDisableParallelApexTesting,
			EnableChatter,
			EnableCommunityWorkspaces,
			NetworksEnabled,
			DeleteDCIWithFiles,
			EnableEnhancedNotes,
			EnableForecasts,
			EnableS1DesktopEnabled,
			EnableS1EncryptedStoragePref2,
			EnableAcademicOperations,
			EnableAlumniRelations,
			EnableBenefitManagementPreference,
			EnableBenefitAndGoalSharingPref,
			EnableCarePlansPreference,
			EnableDiscoveryFrameworkMetadata,
			EnableEducationCloud,
			EnableFundraising,
			EnableGroupMembershipPref,
			EnableIndustriesAssessment,
			EnableInteractionSummaryPref,
			EnableInteractionSummaryRoleHierarchy,
			EnableStudentSuccess,
			EnableInterestTagging,
			EnableMiddleName,
			EnableNameSuffix,
			EnableOpportunityTeam,
			EnableRevenueSchedule,
			EnableEnhancedPermsetMgmt,
			EnableEnhancedProfileMgmt,
			EnableNewProfileUI,
			PermsetsInFieldCreation,
		},
	}

	settingSet := make(map[ScratchSetting]bool)

	for _, product := range products {
		if ps, ok := productSettings[product]; ok {
			for _, s := range ps {
				settingSet[s] = true
			}
		}
	}

	for _, setting := range settings {
		settingSet[setting] = true
	}

	uniqueSettings := make([]string, 0, len(settingSet))
	for setting := range settingSet {
		uniqueSettings = append(uniqueSettings, ScratchSettingIds[setting][0])
	}

	return uniqueSettings
}

func scratchLogin(scratchUser string, features []string, edition string, settings []string, namespace string, release string, duration int) {
	if force == nil {
		ErrorAndExit("You must be logged into a Dev Hub org to authenticate as a scratch org user.")
	}
	fmt.Fprintln(os.Stderr, "Creating new Scratch Org...")
	scratchOrgId, err := force.CreateScratchOrgWithDuration(scratchUser, features, edition, settings, namespace, release, duration)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	session, err := force.ForceLoginNewScratch(scratchOrgId)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	scratchForce := NewForce(&session)
	// Use the scratch org's latest supported API version for subsequent
	// operations. Settings deployment in particular relies on metadata fields
	// that don't exist in older API versions.
	if latest, err := scratchForce.LatestApiVersion(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: unable to determine scratch org API version: %s\n", err)
	} else {
		SetApiVersion(latest)
		scratchForce.Metadata.ApiVersion = latest
		session.SessionOptions.ApiVersion = latest
	}
	_, err = ForceSaveLogin(session, os.Stderr)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	if len(settings) > 0 {
		fmt.Fprintln(os.Stderr, "Deploying settings...")
		err = scratchForce.DeploySettings(settings)
		if err != nil {
			ErrorAndExit(fmt.Sprintf("Settings deployment failed: %s", err.Error()))
		}
	}
}

func oauthLogin(endpoint string, skipLogin bool, port int) {
	var err error
	if skipLogin {
		_, err = ForceLoginAtEndpointWithPromptAndSaveWithPort(endpoint, os.Stdout, "consent", port)
	} else {
		_, err = ForceLoginAtEndpointAndSaveWithPort(endpoint, os.Stdout, port)
	}
	if err != nil {
		ErrorAndExit(err.Error())
	}
}

var errDeviceFlowRequiresClientId = errors.New("--device-flow requires --connected-app-client-id: Salesforce allows the device flow only with a local external client app. Create one in the org with the device flow enabled, and pass its consumer key")

// validateDeviceFlowClientId requires a consumer key for the device flow.
// Salesforce allows the device flow only with a local external client app, so
// the default connected app cannot be used.
func validateDeviceFlowClientId(clientId string) error {
	if clientId == "" {
		return errDeviceFlowRequiresClientId
	}
	return nil
}

func oauthDeviceFlowLogin(endpoint string, skipLogin bool, port int) {
	var err error
	if skipLogin {
		_, err = ForceLoginAtEndpointAndSaveDeviceFlow(endpoint, os.Stdout, "consent")
	} else {
		_, err = ForceLoginAtEndpointAndSaveDeviceFlow(endpoint, os.Stdout, "login")
	}
	if err != nil {
		ErrorAndExit(err.Error())
	}
}

func jwtLogin(endpoint, username, keyfile string) {
	assertion, err := JwtAssertionForEndpoint(endpoint, username, keyfile, ClientId)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	_, err = ForceLoginAtEndpointAndSaveJWT(endpoint, assertion, os.Stdout)
	if err != nil {
		ErrorAndExit(err.Error())
	}
}

func clientCredentialsLogin(endpoint, clientId, clientSecret string) {
	_, err := ForceLoginAtEndpointAndSaveClientCredentials(endpoint, clientId, clientSecret, os.Stdout)
	if err != nil {
		ErrorAndExit(err.Error())
	}
}

func passwordLogin(endpoint, clientId, clientSecret, username, password string) {
	if len(password) == 0 {
		var err error
		password, err = speakeasy.Ask("Password: ")
		if err != nil {
			ErrorAndExit(err.Error())
		}
	}
	_, err := ForceLoginAtEndpointAndSavePasswordFlow(endpoint, clientId, clientSecret, username, password, os.Stdout)
	if err != nil {
		ErrorAndExit(err.Error())
	}
}

func soapLogin(endpoint, username, password string) {
	if len(password) == 0 {
		var err error
		password, err = speakeasy.Ask("Password: ")
		if err != nil {
			ErrorAndExit(err.Error())
		}
	}
	fmt.Fprintln(os.Stderr, "Warning: SOAP login will no longer be supported after Summer '27. Pass --connected-app-client-id and --connected-app-client-secret to use the OAuth Username/Password flow instead.")
	_, err := ForceLoginAtEndpointAndSaveSoap(endpoint, username, password, os.Stdout)
	if err != nil {
		ErrorAndExit(err.Error())
	}
}

func getEndpoint(cmd *cobra.Command) string {
	endpoint := "https://login.salesforce.com"
	// If no instance specified, try to get last endpoint used
	var instance string
	if instance, _ = cmd.Flags().GetString("instance"); instance == "" {
		currentEndpointUrl, err := currentEndpointUrl()
		if err == nil && currentEndpointUrl != "" {
			endpoint = currentEndpointUrl
		}
	}
	switch instance {
	case "login":
		endpoint = "https://login.salesforce.com"
	case "test":
		endpoint = "https://test.salesforce.com"
	case "pre":
		endpoint = "https://prerelna1.salesforce.com"
	case "mobile1":
		endpoint = "https://mobile1.t.pre.salesforce.com"
	default:
		if instance != "" {
			//need to determine the form of the endpoint
			uri, err := url.Parse(instance)
			if err != nil {
				ErrorAndExit("Unable to parse endpoint: %s", instance)
			}
			// Could be short hand?
			if uri.Host == "" {
				uri, err = url.Parse(fmt.Sprintf("https://%s", instance))
				//fmt.Println(uri)
				if err != nil {
					ErrorAndExit("Could not identify host: %s", instance)
				}
			}
			endpoint = uri.Scheme + "://" + uri.Host

			fmt.Println("Loaded Endpoint: (" + endpoint + ")")
		}
	}
	return endpoint
}

func currentEndpoint() (endpoint ForceEndpoint, customUrl string, err error) {
	Log.Info("Deprecated call to CurrentEndpoint.  Use CurrentEndpointUrl.")
	creds, err := ActiveCredentials(false)
	if err != nil {
		return
	}
	endpoint = creds.ForceEndpoint
	customUrl = creds.InstanceUrl
	return
}

func currentEndpointUrl() (endpoint string, err error) {
	creds, err := ActiveCredentials(false)
	if err != nil {
		return "", err
	}
	return creds.EndpointUrl, nil
}

func selectApiVersion(cmd *cobra.Command) {
	if apiVersion, _ := cmd.Flags().GetString("api-version"); apiVersion != "" {
		// Todo verify format of version is 30.0
		SetApiVersion(apiVersion)
	} else {
		//override api version in case of a new login
		SetApiVersion(DefaultApiVersionNumber)
	}
}
