package cmd

import (
	"context"
	"fmt"
	"gas/internal/helpers"
	"gas/internal/resources"
	"os"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type ResourceProcessorOkChan = chan bool

type ResourceProcessors map[string]func(arg interface{}, resourceProcessOkChan ResourceProcessorOkChan)

var resourceProcessors ResourceProcessors = make(ResourceProcessors)

func init() {
	resourceProcessors["cloudflare-kv:create"] = func(arg interface{}, resourceProcessorOkChan ResourceProcessorOkChan) {
		api, err := cloudflare.NewWithAPIToken(os.Getenv("CLOUDFLARE_API_TOKEN"))
		if err != nil {
			fmt.Println("Error:", err)
			resourceProcessorOkChan <- false
			return
		}

		req := cloudflare.CreateWorkersKVNamespaceParams{Title: "test_namespace2"}
		response, err := api.CreateWorkersKVNamespace(context.Background(), cloudflare.AccountIdentifier(os.Getenv("CLOUDFLARE_ACCOUNT_ID")), req)
		if err != nil {
			fmt.Println("Error:", err)
			resourceProcessorOkChan <- false
			return
		}

		fmt.Println(response)
		resourceProcessorOkChan <- true
	}
}

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy resources to the cloud",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Deploying resources to the cloud")

		currResourceContainerSubdirPaths, err := resources.GetContainerSubdirPaths(viper.GetString("resourceContainerDir"))
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
			return
		}

		err = resources.ValidateContainerSubdirContents(currResourceContainerSubdirPaths)
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
			return
		}

		currResourceIndexBuildFilePaths, err := resources.GetIndexBuildFilePaths(currResourceContainerSubdirPaths)
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
			return
		}

		currResourceIndexBuildFileConfigs, err := resources.GetIndexBuildFileConfigs(currResourceIndexBuildFilePaths)
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
			return
		}

		currResourcePackageJsons, err := resources.GetPackageJsons(currResourceContainerSubdirPaths)
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
			return
		}

		currResourcePackageJsonNameToTrue := resources.SetPackageJsonNameToTrue(currResourcePackageJsons)

		currResourcePackageJsonNameToID := resources.SetPackageJsonNameToID(currResourcePackageJsons, currResourceIndexBuildFileConfigs)

		currResourceDependencyIDs := resources.SetDependencyIDs(currResourcePackageJsons, currResourcePackageJsonNameToID, currResourcePackageJsonNameToTrue)

		currResourceIDToData := resources.SetIDToData(currResourceIndexBuildFileConfigs, currResourceDependencyIDs)

		resourceIDToInDegrees := resources.SetIDToInDegrees(currResourceIDToData)

		resourceIDsWithInDegreesOfZero := resources.SetIDsWithInDegreesOf(resourceIDToInDegrees, 0)

		// TODO: Might not need this
		resourceIDs := resources.SetIDs(currResourceIDToData)

		resourceIDToIntermediateIDs := resources.SetIDToIntermediateIDs(currResourceIDToData)

		resourceIDToGroup := resources.SetIDToGroup(resourceIDsWithInDegreesOfZero, resourceIDToIntermediateIDs)

		depthToResourceID := resources.SetDepthToResourceID(resourceIDs, currResourceIDToData, resourceIDsWithInDegreesOfZero)

		resourceIDToDepth := resources.SetIDToDepth(depthToResourceID)

		groupToDepthToResourceIDs := resources.SetGroupToDepthToResourceIDs(resourceIDToGroup, resourceIDToDepth)

		// TODO: json path can be configged?
		// TODO: Or implement up -> driver -> local | gh in the config?
		resourcesUpJsonPath := "gas.up.json"

		isResourcesUpJsonPresent := helpers.IsFilePresent(resourcesUpJsonPath)

		if !isResourcesUpJsonPresent {
			err = helpers.WriteFile(resourcesUpJsonPath, "{}")
			if err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
				return
			}
		}

		resourcesUpJson, err := resources.GetUpJson(resourcesUpJsonPath)
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
			return
		}

		resourceIDToState := resources.SetIDToStateMap(resourcesUpJson, currResourceIDToData)

		stateToResourceIDs := resources.SetStateToResourceIDs(resourceIDToState)

		hasResourceIDsToDeploy := resources.HasIDsToDeploy(stateToResourceIDs)

		if !hasResourceIDsToDeploy {
			fmt.Println("No resource changes to deploy")
			return
		}

		err = deploy(resourceIDToState, resourceIDToGroup, resourceIDToDepth, groupToDepthToResourceIDs, currResourceIDToData)

		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
			return
		}

		fmt.Println("Deployment successful")
	},
}

func deploy(resourceIDToState resources.ResourceIDToState, resourceIDToGroup resources.ResourceIDToGroup, resourceIDToDepth resources.ResourceIDToDepth, groupToDepthToResourceIDs resources.GroupToDepthToResourceIDs, currResourceIDToData resources.ResourceIDToData) error {
	resources.LogIDPreDeploymentStates(groupToDepthToResourceIDs, resourceIDToState)

	resourceIDToDeployState := resources.UpdateIDToDeployStateOfPending(resourceIDToState)

	err := deployGroups(resourceIDToDeployState, resourceIDToState, groupToDepthToResourceIDs, currResourceIDToData, resourceIDToDepth, resourceIDToGroup)

	if err != nil {
		return err
	}

	return nil
}

func deployGroups(resourceIDToDeployState resources.ResourceIDToDeployState, resourceIDToState resources.ResourceIDToState, groupToDepthToResourceIDs resources.GroupToDepthToResourceIDs, currResourceIDToData resources.ResourceIDToData, resourceIDToDepth resources.ResourceIDToDepth, resourceIDToGroup resources.ResourceIDToGroup) error {
	groupsWithStateChanges := resources.SetGroupsWithStateChanges(resourceIDToGroup, resourceIDToState)

	groupsToResourceIDs := resources.SetGroupToResourceIDs(resourceIDToGroup)

	groupToHighestDeployDepth := resources.SetGroupToHighestDeployDepth(
		resourceIDToDepth,
		resourceIDToState,
		groupsWithStateChanges,
		groupsToResourceIDs,
	)

	numOfGroupsToDeploy := len(groupsWithStateChanges)

	deployGroupOkChan := make(DeployGroupOkChan)

	for _, group := range groupsWithStateChanges {
		go deployGroup(group, deployGroupOkChan, resourceIDToDeployState, resourceIDToState, groupToHighestDeployDepth, groupToDepthToResourceIDs, currResourceIDToData, resourceIDToDepth, groupsToResourceIDs)
	}

	numOfGroupsDeployedOk := 0
	numOfGroupsDeployedErr := 0

	for groupDeployedOk := range deployGroupOkChan {
		if groupDeployedOk {
			numOfGroupsDeployedOk++
		} else {
			numOfGroupsDeployedErr++
		}

		numOfGroupsFinishedDeploying := numOfGroupsDeployedOk + numOfGroupsDeployedErr

		if numOfGroupsFinishedDeploying == numOfGroupsToDeploy {
			if numOfGroupsDeployedErr > 0 {
				return fmt.Errorf("deployment failed")
			}
			break
		}
	}

	return nil
}

type DeployGroupOkChan chan bool

func deployGroup(group int, deployGroupOkChan DeployGroupOkChan, resourceIDToDeployState resources.ResourceIDToDeployState, resourceIDToState resources.ResourceIDToState, groupToHighestDeployDepth resources.GroupToHighestDeployDepth, groupToDepthToResourceIDs resources.GroupToDepthToResourceIDs, currResourceIDToData resources.ResourceIDToData, resourceIDToDepth resources.ResourceIDToDepth, groupsToResourceIDs resources.GroupToResourceIDs) {

	deployResourceOkChan := make(DeployResourceOkChan)

	highestGroupDeployDepth := groupToHighestDeployDepth[group]

	initialGroupResourceIDsToDeploy := resources.SetInitialGroupIDsToDeploy(highestGroupDeployDepth, group, groupToDepthToResourceIDs, currResourceIDToData, resourceIDToDeployState)

	for _, resourceID := range initialGroupResourceIDsToDeploy {
		depth := resourceIDToDepth[resourceID]
		go deployResource(deployResourceOkChan, group, depth, resourceID, resourceIDToDeployState, resourceIDToState)
	}

	numOfResourcesInGroupToDeploy := resources.SetNumInGroupToDeploy(
		groupsToResourceIDs,
		resourceIDToState,
		group,
	)

	numOfResourcesDeployedOk := 0
	numOfResourcesDeployedErr := 0
	numOfResourcesDeployedCanceled := 0

	for resourceDeployedOk := range deployResourceOkChan {
		if resourceDeployedOk {
			numOfResourcesDeployedOk++
		} else {
			numOfResourcesDeployedErr++
			// Cancel PENDING resources.
			// Check for 0 because resources should only
			// be canceled one time.
			if numOfResourcesDeployedCanceled == 0 {
				numOfResourcesDeployedCanceled = resources.UpdateIDToDeployStateOfCanceled(resourceIDToDeployState)
			}
		}

		numOfResourcesInFinalDeployState := numOfResourcesDeployedOk + numOfResourcesDeployedErr + numOfResourcesDeployedCanceled

		if numOfResourcesInFinalDeployState == int(numOfResourcesInGroupToDeploy) {
			if numOfResourcesDeployedErr == 0 {
				deployGroupOkChan <- true
			} else {
				deployGroupOkChan <- false
			}
			return
		} else {
			for _, resourceID := range groupsToResourceIDs[group] {
				if resourceIDToDeployState[resourceID] == resources.DeployState("PENDING") {
					shouldDeployResource := true

					// Is resource dependent on another deploying resource?
					for _, dependencyID := range currResourceIDToData[resourceID].Dependencies {
						activeStates := map[resources.DeployState]bool{
							resources.DeployState(resources.CREATE_IN_PROGRESS): true,
							resources.DeployState(resources.DELETE_IN_PROGRESS): true,
							resources.DeployState(resources.PENDING):            true,
							resources.DeployState(resources.UPDATE_IN_PROGRESS): true,
						}

						dependencyIDDeployState := resourceIDToDeployState[dependencyID]

						if activeStates[dependencyIDDeployState] {
							shouldDeployResource = false
							break
						}
					}

					if shouldDeployResource {
						depth := resourceIDToDepth[resourceID]
						go deployResource(deployResourceOkChan, group, depth, resourceID, resourceIDToDeployState, resourceIDToState)
					}
				}
			}
		}
	}
}

type DeployResourceOkChan chan bool

func deployResource(deployResourceOkChan DeployResourceOkChan, group int, depth int, resourceID string, resourceIDToDeployState resources.ResourceIDToDeployState, resourceIDToState resources.ResourceIDToState) {
	resources.UpdateIDToDeployStateOnStart(resourceIDToDeployState, resourceIDToState, resourceID)

	timestamp := time.Now().UnixMilli()

	resources.LogIDDeployState(group, depth, resourceID, timestamp, resourceIDToDeployState)

	resourceProcessorOkChan := make(ResourceProcessorOkChan)

	go resourceProcessors["cloudflare-kv:create"]("test", resourceProcessorOkChan)

	if <-resourceProcessorOkChan {
		resources.UpdateResourceIDToDeployStateOnOk(
			resourceIDToDeployState,
			resourceID,
		)

		timestamp = time.Now().UnixMilli()

		resources.LogIDDeployState(
			group,
			depth,
			resourceID,
			timestamp,
			resourceIDToDeployState,
		)

		deployResourceOkChan <- true

		return
	}

	resources.UpdateResourceIDToDeployStateOnErr(
		resourceIDToDeployState,
		resourceID,
	)

	timestamp = time.Now().UnixMilli()

	resources.LogIDDeployState(
		group,
		depth,
		resourceID,
		timestamp,
		resourceIDToDeployState,
	)

	deployResourceOkChan <- false
}
