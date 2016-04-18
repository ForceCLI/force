package salesforce

func (apps ForceConnectedApps) Len() int {
	return len(apps)
}

func (apps ForceConnectedApps) Less(i, j int) bool {
	return apps[i].Name < apps[j].Name
}

func (apps ForceConnectedApps) Swap(i, j int) {
	apps[i], apps[j] = apps[j], apps[i]
}

type ForceSobjectFields []interface{}

func (f ForceSobjectFields) Len() int {
	return len(f)
}

func (f ForceSobjectFields) Less(i, j int) bool {
	return f[i].(map[string]interface{})["name"].(string) < f[j].(map[string]interface{})["name"].(string)
}

func (f ForceSobjectFields) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
}
