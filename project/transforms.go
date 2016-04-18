package project

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"

	"github.com/heroku/force/salesforce"
	"github.com/heroku/force/util"
)

// IsNewFlowVersionsOnlyTransformRequired lets consumer code (`import` command)
// know if use of the TransformDeployToIncludeNewFlowVersionsOnly is indicated.
// This is useful because the transform is expensive because it requires
// fetching metadata from the target environment beforehand.
func IsNewFlowVersionsOnlyTransformRequired(sourceMetadata map[string][]byte) (transformRequired bool) {
	flowDefinitions := salesforce.EnumerateMetadataByType(sourceMetadata, "FlowDefinition", "flowDefinitions", "flowDefinition", "")
	flowVersions := salesforce.EnumerateMetadataByType(sourceMetadata, "Flow", "flows", "flow", "")
	transformRequired = len(flowDefinitions.Members) > 0 || len(flowVersions.Members) > 0
	return
}

// TransformDeployToIncludeNewFlowVersionsOnly allows you to deploy only those
// flows that have changed, and also that are active.  This is useful because
// stock Salesforce "helpfully" tries to enforce a development process by
// ensuring version control on flows: that is, once a flow is deployed and
// activated, it can not be replaced, only superceded.  Unfortunately, this ends
// up self-defeating because it stymies attempts to track Salesforce metadata
// using external change management tools.  This transform works around this by
// determining which versions have already been deployed and removes them from
// the package.
func TransformDeployToIncludeNewFlowVersionsOnly(sourceMetadata map[string][]byte, targetCurrentMetadata map[string][]byte) (transformedSourceMetadata map[string][]byte) {
	// make a copy of the sourceMetadata so that we can return it without
	// modifying the source at all.
	transformedSourceMetadata = make(map[string][]byte)
	for k, v := range sourceMetadata {
		transformedSourceMetadata[k] = v
	}

	// MetadataFlowState describes the state of a given flow in an environment.
	type MetadataFlowDefinitionState struct {
		ActiveVersion uint64
		Name          string

		ActiveContent salesforce.ForceMetadataItem
		AllVersions   map[uint64]salesforce.ForceMetadataItem
	}

	// EnvironmentFlowState semantically describes what flows and versions are present in an environment,
	// which are active.
	type EnvironmentFlowState struct {
		EnvironmentName string
		ActiveFlows     map[string]MetadataFlowDefinitionState
		InactiveFlows   map[string]MetadataFlowDefinitionState
	}

	determineEnvironmentState := func(metadataFiles salesforce.ForceMetadataFiles, environmentName string) EnvironmentFlowState {
		flowDefinitions := salesforce.EnumerateMetadataByType(metadataFiles, "FlowDefinition", "flowDefinitions", "flowDefinition", "")

		state := EnvironmentFlowState{
			EnvironmentName: environmentName,
			ActiveFlows:     make(map[string]MetadataFlowDefinitionState),
			InactiveFlows:   make(map[string]MetadataFlowDefinitionState),
		}
		// First, determine what flows are active.
		for _, item := range flowDefinitions.Members {
			var res salesforce.FlowDefinition

			if err := xml.Unmarshal(item.Content, &res); err != nil {
				util.ErrorAndExit(err.Error())
			}

			if res.ActiveVersionNumber != 0 {
				state.ActiveFlows[item.Name] = MetadataFlowDefinitionState{
					ActiveVersion: res.ActiveVersionNumber,
					Name:          item.Name,
					AllVersions:   make(map[uint64]salesforce.ForceMetadataItem),
				}
			} else {
				state.InactiveFlows[item.Name] = MetadataFlowDefinitionState{
					ActiveVersion: res.ActiveVersionNumber,
					Name:          item.Name,
					AllVersions:   make(map[uint64]salesforce.ForceMetadataItem),
				}
			}
		}

		// now, enumerate the flows themselves and index them in:
		flowVersions := salesforce.EnumerateMetadataByType(metadataFiles, "Flow", "flows", "flow", "")
		for _, version := range flowVersions.Members {

			// the version number is indicated by a normalized naming convention in the entries rendered by the
			// Metadata API: -$version appended to the name
			//   MyFlow-4

			nameFragments := strings.Split(version.Name, "-")
			name := nameFragments[0]
			versionNumber, err := strconv.ParseUint(nameFragments[len(nameFragments)-1], 10, 64)
			if err != nil {
				util.ErrorAndExit(err.Error())
			}

			if flowDefinition, present := state.InactiveFlows[name]; present {
				flowDefinition.AllVersions[versionNumber] = version
			} else if flowDefinition, present := state.ActiveFlows[name]; present {
				flowDefinition.AllVersions[versionNumber] = version
				// set the FlowContent value for the version we have here if it's indeed the active one:
				if state.ActiveFlows[name].ActiveVersion == versionNumber {
					flowDefinition.ActiveContent = version
				}
				// alas because golang is silly and prevents us from mutating stuff in maps
				// while being an imperative language, we have to copy the value, mutate it, and re-insert it.
				state.ActiveFlows[name] = flowDefinition
			} else {
				fmt.Printf("Warning: found a flow version instance on %s for which we have no flow definition at all, consider cleaning it up (we can't determine if it can be deployed or not): %s\n", environmentName, name)
			}
		}

		return state
	}

	targetState := determineEnvironmentState(targetCurrentMetadata, "target")

	sourceState := determineEnvironmentState(sourceMetadata, "source")

	// now, index the state of the flows we just determined, by using their full path names.
	// this allows us to use them to filter the transformedSourceMetadata itself.

	activeFlowsInSourceByCompletePath := make(map[string]MetadataFlowDefinitionState)
	for _, flowState := range sourceState.ActiveFlows {
		activeFlowsInSourceByCompletePath[flowState.ActiveContent.CompletePath] = flowState
	}

	activeFlowsInTargetByCompletePath := make(map[string]MetadataFlowDefinitionState)
	for _, flowState := range targetState.ActiveFlows {
		activeFlowsInTargetByCompletePath[flowState.ActiveContent.CompletePath] = flowState
	}

	// now, we can finally transform the metadata only include flows that are
	// active in the source (and only that version) if they aren't already
	// active in the target.
	for fileName := range transformedSourceMetadata {
		if _, alreadyDeployed := activeFlowsInTargetByCompletePath[fileName]; alreadyDeployed {
			// already deployed, don't need it.
			fmt.Printf("Not going to deploy '%s' because it's already deployed and active on our target!\n", fileName)
			delete(transformedSourceMetadata, fileName)
		}

		// TODO alas, this negative filtering logic is a bit difficult to follow.
	}

	return
}
