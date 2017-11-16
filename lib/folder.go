package lib

import (
	"fmt"
)

type FolderType string
type FolderId string
type FolderName string
type NamespacePrefix string

type Folders map[FolderId]FolderName
type FolderedMetadata map[FolderType]Folders

func (force *Force) GetAllFolders() (folders FolderedMetadata, err error) {
	folderResult, err := force.Query(fmt.Sprintf("%s", "SELECT Id, Type, NamespacePrefix, DeveloperName from Folder Where Type in ('Dashboard', 'Document', 'Email', 'Report')"), QueryOptions{IsTooling: false})
	if err != nil {
		return
	}
	folders = make(FolderedMetadata)
	for _, folder := range folderResult.Records {
		if folder["DeveloperName"] != nil {
			folderType := FolderType(folder["Type"].(string))
			m, ok := folders[folderType]
			if !ok {
				m = make(Folders)
				folders[folderType] = m
			}
			folderFullName := folder["DeveloperName"].(string)
			if folder["NamespacePrefix"] != nil {
				folderFullName = fmt.Sprintf("%s__%s", folder["NamespacePrefix"].(string), folderFullName)
			}
			m[FolderId(folder["Id"].(string))] = FolderName(folderFullName)
		}
	}
	return
}

func (force *Force) GetMetadataInFolders(metadataType FolderType, folders Folders) (metadataItems []string, err error) {
	var queryString string
	if metadataType == "Report" {
		queryString = "SELECT Id, OwnerId, DeveloperName, NamespacePrefix FROM Report"
	} else {
		queryString = "SELECT Id, DeveloperName, Folder.DeveloperName, Folder.NamespacePrefix, NamespacePrefix FROM " + string(metadataType)
	}
	queryResult, err := force.Query(fmt.Sprintf("%s", queryString), QueryOptions{IsTooling: false})
	if err != nil {
		return
	}
	metadataItems = make([]string, 1, 1000)
	metadataItems[0] = "*"
	for _, folderName := range folders {
		metadataItems = append(metadataItems, string(folderName))
	}

	for _, metadataItem := range queryResult.Records {
		folderName := ""
		if metadataType == "Report" {
			ownerId, _ := metadataItem["OwnerId"].(string)
			folderId := FolderId(ownerId)
			folderName = string(folders[folderId])
		} else {
			folderData, _ := metadataItem["Folder"].(map[string]interface{})
			if folderData != nil {
				folderName = folderData["DeveloperName"].(string)
				if folderData["NamespacePrefix"] != nil {
					folderName = fmt.Sprintf("%s__%s", folderData["NamespacePrefix"].(string), folderName)
				}
			}
		}
		itemName := metadataItem["DeveloperName"].(string)
		if metadataItem["NamespacePrefix"] != nil {
			itemName = fmt.Sprintf("%s__%s", metadataItem["NamespacePrefix"].(string), itemName)
		}
		if folderName != "" {
			metadataItems = append(metadataItems, folderName+"/"+itemName)
		}
	}
	return
}
