## force login scratch

Create scratch org and log in

### Synopsis

Create scratch org and log in

Available Features:
  AccountingSubledgerGrowthEdition    - Enables Accounting Subledger Growth Edition
  AccountingSubledgerUser             - Enables Accounting Subledger user licenses
  AdmissionsConnectUser               - Enables Admissions Connect user licenses
  AdvisorLinkFeature                  - Enables Advisor Link
  AdvisorLinkPathwaysFeature          - Enables Advisor Link Pathways
  AnalyticsAdminPerms                 - Enables CRM Analytics admin permissions
  AnalyticsQueryService               - Enables Analytics Query Service
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
  Fundraising                         - Enables Fundraising
  HealthCloudAddOn                    - Enables Health Cloud add-on
  HealthCloudUser                     - Enables Health Cloud user licenses
  IndustriesActionPlan                - Enables Industries Action Plans
  IndustriesSalesExcellenceAddOn      - Enables Industries Sales Excellence Add-On
  IndustriesServiceExcellenceAddOn    - Enables Industries Service Excellence Add-On
  InsightsPlatform                    - Enables Insights Platform
  Knowledge                           - Enables Salesforce Knowledge
  LightningScheduler                  - Enables Lightning Scheduler
  LightningServiceConsole             - Enables Lightning Service Console
  LiveAgent                           - Enables Live Agent (Chat)
  LiveMessage                         - Enables LiveMessage (SMS/MMS messaging)
  MarketingUser                       - Enables Marketing User licenses
  OmniStudioDesigner                  - Enables OmniStudio Designer
  OmniStudioRuntime                   - Enables OmniStudio Runtime
  OrderManagement                     - Enables Salesforce Order Management
  OrderSaveLogicEnabled               - Enables order save behavior logic
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
  WavePlatform                        - Enables Wave Platform (CRM Analytics)

Available Products:
  b2bcommerce      - B2B Commerce (enables B2BCommerce, OrderManagement features and commerceEnabled, enableOrders, enableEnhancedCommerceOrders settings)
  communities      - Experience Cloud (enables Communities feature and networksEnabled setting)
  crmanalytics     - CRM Analytics (enables AnalyticsAdminPerms, WavePlatform, InsightsPlatform, EinsteinAnalyticsPlus, EinsteinBuilderFree, DevelopmentWave)
  educationcloud   - Education Cloud (enables EducationCloud, PersonAccounts, Communities, Knowledge, and many other Education Cloud features with a comprehensive set of industries/education settings)
  fsc              - Financial Services Cloud (enables PersonAccounts, ContactsToMultipleAccounts, FinancialServicesUser)
  healthcloud      - Health Cloud (enables HealthCloudAddOn, HealthCloudUser)
  knowledge        - Salesforce Knowledge (enables Knowledge feature and enableKnowledge, enableLightningKnowledge settings)
  liveagent        - Live Agent (enables LiveAgent feature and enableLiveAgent setting)
  messaging        - Messaging (enables EmbeddedServiceMessaging, LiveMessage, BYOOTT features)
  revenuecloud     - Revenue Cloud (enables CoreCpq, BillingAdvanced, UsageManagement, DocGen, Einstein1AIPlatform, OrderManagement, Communities, PartnerCommunity, CustomerCommunityPlus, EnableSetPasswordInApi, OrderSaveLogicEnabled features and a comprehensive set of billing/order/quote/pricing/rating settings)

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
  enableDiscoveryFrameworkMetadata      - Enable Discovery Framework Metadata (IndustriesSettings)
  enableEducationCloud                  - Enable Education Cloud (IndustriesSettings)
  enableFundraising                     - Enable Fundraising (IndustriesSettings)
  enableGroupMembershipPref             - Enable Group Membership (IndustriesSettings)
  enableIndustriesAssessment            - Enable Industries Assessment (IndustriesSettings)
  enableInteractionSummaryPref          - Enable Interaction Summary (IndustriesSettings)
  enableInteractionSummaryRoleHierarchy - Enable Interaction Summary Role Hierarchy (IndustriesSettings)
  enableStudentSuccess                  - Enable Student Success (IndustriesSettings)
  enableInterestTagging                 - Enable Interest Tagging (InterestTaggingSettings)
  enableMiddleName                      - Enable Middle Name (NameSettings)
  enableNameSuffix                      - Enable Name Suffix (NameSettings)
  enableRevenueSchedule                 - Enable Revenue Schedule (ProductSettings)
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
  force login scratch --product revenuecloud
  force login scratch --release preview
  force login scratch --release previous
  force login scratch --duration 14

```
force login scratch [flags]
```

### Options

```
      --duration int              number of days before the scratch org expires (1-30) (default 7)
      --edition edition           scratch org edition; see command help for available editions (default Developer)
      --feature feature           feature to enable (can be specified multiple times); see command help for available features (default [])
  -h, --help                      help for scratch
      --namespace string          namespace for the scratch org
      --product product           product shortcut for features (can be specified multiple times); see command help for available products (default [])
      --quantity stringToString   override default quantity for features (e.g., FinancialServicesUser=5); default quantity is 10 (default [])
      --release release           Salesforce release for scratch org: preview (next release) or previous
      --setting setting           setting to enable (can be specified multiple times); see command help for available settings (default [])
      --username string           username for scratch org user
```

### Options inherited from parent commands

```
  -a, --account username    account username to use
  -V, --apiversion string   API version to use
      --config string       config directory to use (default: .force)
```

### SEE ALSO

* [force login](force_login.md)	 - Log into Salesforce and store a session token

