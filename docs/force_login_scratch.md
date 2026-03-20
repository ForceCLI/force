## force login scratch

Create scratch org and log in

### Synopsis

Create scratch org and log in

Available Features:
  AnalyticsAdminPerms                 - Enables CRM Analytics admin permissions
  ApexUserModeWithPermset             - Enables Apex code to run in user mode with a permission set session
  B2BCommerce                         - Enables B2B Commerce
  Communities                         - Enables Experience Cloud (Communities)
  ContactsToMultipleAccounts          - Allows a single Contact to be associated with multiple Accounts
  DevelopmentWave                     - Enables CRM Analytics development features
  EinsteinAnalyticsPlus               - Enables Einstein Analytics Plus
  EinsteinBuilderFree                 - Enables Einstein Builder Free
  EventLogFile                        - Enables Event Log File
  FinancialServicesUser               - Enables Financial Services Cloud user licenses (requires quantity, default: 10)
  HealthCloudAddOn                    - Enables Health Cloud add-on
  HealthCloudUser                     - Enables Health Cloud user licenses
  InsightsPlatform                    - Enables Insights Platform
  OrderManagement                     - Enables Salesforce Order Management
  PersonAccounts                      - Enables Person Accounts (B2C account model)
  ScvMultipartyAndConsult             - Enables Service Cloud Voice multiparty and consult (requires quantity, default: 10)
  ServiceCloud                        - Enables Service Cloud
  ServiceCloudVoicePartnerTelephony   - Enables Service Cloud Voice Partner Telephony (requires quantity 1-50, default: 10)
  StateAndCountryPicklist             - Enables State and Country Picklists for standard address fields
  WavePlatform                        - Enables Wave Platform (CRM Analytics)

Available Products:
  b2bcommerce  - B2B Commerce (enables B2BCommerce, OrderManagement features and commerceEnabled, enableOrders, enableEnhancedCommerceOrders settings)
  communities  - Experience Cloud (enables Communities feature and networksEnabled setting)
  crmanalytics - CRM Analytics (enables AnalyticsAdminPerms, WavePlatform, InsightsPlatform, EinsteinAnalyticsPlus, EinsteinBuilderFree, DevelopmentWave)
  fsc          - Financial Services Cloud (enables PersonAccounts, ContactsToMultipleAccounts, FinancialServicesUser)
  healthcloud  - Health Cloud (enables HealthCloudAddOn, HealthCloudUser)

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
  enableEnhancedNotes - Enable Enhanced Notes
  enableQuote         - Enable Quotes
  networksEnabled     - Enable Experience Cloud (Communities)
  commerceEnabled     - Enable Commerce
  enableApexApprovalLockUnlock - Allow Apex to lock/unlock approval processes
  permsetsInFieldCreation - Allow assigning permission sets during field creation
  enableLightningPreviewPref - Enable Lightning Experience preview pref
  enableOrders - Enable Orders
  enableEnhancedCommerceOrders - Enable Enhanced Commerce Orders

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

