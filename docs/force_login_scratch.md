## force login scratch

Create scratch org and log in

### Synopsis

Create scratch org and log in

Available Features:
  AnalyticsAdminPerms                 - Enables CRM Analytics admin permissions
  ApexUserModeWithPermset             - Enables Apex code to run in user mode with a permission set session
  B2BCommerce                         - Enables B2B Commerce
  BillingAdvanced                     - Enables Advanced Billing (Revenue Cloud)
  Communities                         - Enables Experience Cloud (Communities)
  ContactsToMultipleAccounts          - Allows a single Contact to be associated with multiple Accounts
  CustomerCommunityPlus               - Enables Customer Community Plus user licenses
  DevelopmentWave                     - Enables CRM Analytics development features
  DocGen                              - Enables Document Generation
  DSARPortability                     - Enables Data Subject Access Request (DSAR) data portability
  Einstein1AIPlatform                 - Enables Einstein 1 AI Platform
  EinsteinAnalyticsPlus               - Enables Einstein Analytics Plus
  EinsteinBuilderFree                 - Enables Einstein Builder Free
  EnableSetPasswordInApi              - Allows setting passwords via API
  EventLogFile                        - Enables Event Log File
  FinancialServicesUser               - Enables Financial Services Cloud user licenses (requires quantity, default: 10)
  HealthCloudAddOn                    - Enables Health Cloud add-on
  HealthCloudUser                     - Enables Health Cloud user licenses
  InsightsPlatform                    - Enables Insights Platform
  Knowledge                           - Enables Salesforce Knowledge
  LiveAgent                           - Enables Live Agent (Chat)
  OrderManagement                     - Enables Salesforce Order Management
  OrderSaveLogicEnabled               - Enables order save behavior logic
  PartnerCommunity                    - Enables Partner Community user licenses
  PlatformCache                       - Enables Platform Cache
  PersonAccounts                      - Enables Person Accounts (B2C account model)
  PlatformEncryption                  - Enables Shield Platform Encryption
  ProgramManagement                   - Enables Program Management Module (Salesforce.org Nonprofit/Education)
  ScvMultipartyAndConsult             - Enables Service Cloud Voice multiparty and consult (requires quantity, default: 10)
  ServiceCloud                        - Enables Service Cloud
  ServiceCloudVoicePartnerTelephony   - Enables Service Cloud Voice Partner Telephony (requires quantity 1-50, default: 10)
  StateAndCountryPicklist             - Enables State and Country Picklists for standard address fields
  UsageManagement                     - Enables Usage Management (Revenue Cloud)
  WavePlatform                        - Enables Wave Platform (CRM Analytics)

Available Products:
  b2bcommerce  - B2B Commerce (enables B2BCommerce, OrderManagement features and commerceEnabled, enableOrders, enableEnhancedCommerceOrders settings)
  communities  - Experience Cloud (enables Communities feature and networksEnabled setting)
  crmanalytics - CRM Analytics (enables AnalyticsAdminPerms, WavePlatform, InsightsPlatform, EinsteinAnalyticsPlus, EinsteinBuilderFree, DevelopmentWave)
  fsc          - Financial Services Cloud (enables PersonAccounts, ContactsToMultipleAccounts, FinancialServicesUser)
  healthcloud  - Health Cloud (enables HealthCloudAddOn, HealthCloudUser)
  knowledge    - Salesforce Knowledge (enables Knowledge feature and enableKnowledge, enableLightningKnowledge settings)
  liveagent    - Live Agent (enables LiveAgent feature and enableLiveAgent setting)
  revenuecloud - Revenue Cloud (enables CoreCpq, BillingAdvanced, UsageManagement, DocGen, Einstein1AIPlatform, OrderManagement, Communities, PartnerCommunity, CustomerCommunityPlus, EnableSetPasswordInApi, OrderSaveLogicEnabled features and a comprehensive set of billing/order/quote/pricing/rating settings)

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
  force login scratch --product healthcloud
  force login scratch --product knowledge
  force login scratch --product liveagent
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

