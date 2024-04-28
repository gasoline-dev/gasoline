package cmd

import (
	"context"
	"fmt"
	"gas/internal/helpers"
	"gas/internal/resources"
	"gas/internal/validators"
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

		err = validators.ValidateContainerSubdirContents(currResourceContainerSubdirPaths)
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

		currResourceDependencyIDs := resources.SetDependencyIDs(&resources.SetDependencyIDsInput{
			PackageJsons:                currResourcePackageJsons,
			PackageJsonNameToResourceID: currResourcePackageJsonNameToID,
			PackageJsonNameToTrue:       currResourcePackageJsonNameToTrue,
		})

		currResourceIDToData := resources.SetIDToData(currResourceIndexBuildFileConfigs, currResourceDependencyIDs)

		resourceIDToInDegrees := resources.SetIDToInDegrees(currResourceIDToData)

		resourceIDsWithInDegreesOfZero := resources.SetIDsWithInDegreesOf(resourceIDToInDegrees, 0)

		// TODO: Might not need this
		resourceIDs := resources.SetIDs(currResourceIDToData)

		resourceIDToIntermediateIDs := resources.SetIDToIntermediateIDs(currResourceIDToData)

		resourceIDToGroup := resources.SetIDToGroup(resourceIDsWithInDegreesOfZero, resourceIDToIntermediateIDs)

		depthToResourceID := resources.SetDepthToResourceID(&resources.SetDepthToResourceIDInput{
			ResourceIDs:                    resourceIDs,
			ResourceIDToData:               currResourceIDToData,
			ResourceIDsWithInDegreesOfZero: resourceIDsWithInDegreesOfZero,
		})

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

		hasResourceIDsToDeploy := hasResourceIDsToDeploy(stateToResourceIDs)

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
	logResourceIDPreDeployStates(groupToDepthToResourceIDs, resourceIDToState)

	resourceIDToDeployState := updateResourceIDToDeployStateOfPending(resourceIDToState)

	err := deployGroups(resourceIDToDeployState, resourceIDToState, groupToDepthToResourceIDs, currResourceIDToData, resourceIDToDepth, resourceIDToGroup)

	if err != nil {
		return err
	}

	return nil
}

func deployGroups(resourceIDToDeployState resourceIDToDeployState, resourceIDToState resources.ResourceIDToState, groupToDepthToResourceIDs resources.GroupToDepthToResourceIDs, currResourceIDToData resources.ResourceIDToData, resourceIDToDepth resources.ResourceIDToDepth, resourceIDToGroup resources.ResourceIDToGroup) error {
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

func deployGroup(group int, deployGroupOkChan DeployGroupOkChan, resourceIDToDeployState resourceIDToDeployState, resourceIDToState resources.ResourceIDToState, groupToHighestDeployDepth resources.GroupToHighestDeployDepth, groupToDepthToResourceIDs resources.GroupToDepthToResourceIDs, currResourceIDToData resources.ResourceIDToData, resourceIDToDepth resources.ResourceIDToDepth, groupsToResourceIDs resources.GroupToResourceIDs) {

	deployResourceOkChan := make(DeployResourceOkChan)

	highestGroupDeployDepth := groupToHighestDeployDepth[group]

	initialGroupResourceIDsToDeploy := setInitialGroupResourceIDsToDeploy(highestGroupDeployDepth, group, groupToDepthToResourceIDs, currResourceIDToData, resourceIDToDeployState)

	for _, resourceID := range initialGroupResourceIDsToDeploy {
		depth := resourceIDToDepth[resourceID]
		go deployResource(deployResourceOkChan, group, depth, resourceID, resourceIDToDeployState, resourceIDToState)
	}

	numOfResourcesInGroupToDeploy := setNumOfResourcesInGroupToDeploy(
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
				numOfResourcesDeployedCanceled = updateResourceIDToDeployStateOfCanceled(resourceIDToDeployState)
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
				if resourceIDToDeployState[resourceID] == deployState("PENDING") {
					shouldDeployResource := true

					// Is resource dependent on another deploying resource?
					for _, dependencyID := range currResourceIDToData[resourceID].Dependencies {
						activeStates := map[deployState]bool{
							deployState(CREATE_IN_PROGRESS): true,
							deployState(DELETE_IN_PROGRESS): true,
							deployState(PENDING):            true,
							deployState(UPDATE_IN_PROGRESS): true,
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

func deployResource(deployResourceOkChan DeployResourceOkChan, group int, depth int, resourceID string, resourceIDToDeployState resourceIDToDeployState, resourceIDToState resources.ResourceIDToState) {
	updateResourceIDToDeployStateOnStart(resourceIDToDeployState, resourceIDToState, resourceID)

	timestamp := time.Now().UnixMilli()

	logResourceIDDeployState(group, depth, resourceID, timestamp, resourceIDToDeployState)

	resourceProcessorOkChan := make(ResourceProcessorOkChan)

	go resourceProcessors["cloudflare-kv:create"]("test", resourceProcessorOkChan)

	if <-resourceProcessorOkChan {
		updateResourceIDToDeployStateOnOk(
			resourceIDToDeployState,
			resourceID,
		)

		timestamp = time.Now().UnixMilli()

		logResourceIDDeployState(
			group,
			depth,
			resourceID,
			timestamp,
			resourceIDToDeployState,
		)

		deployResourceOkChan <- true

		return
	}

	updateResourceIDToDeployStateOnErr(
		resourceIDToDeployState,
		resourceID,
	)

	timestamp = time.Now().UnixMilli()

	logResourceIDDeployState(
		group,
		depth,
		resourceID,
		timestamp,
		resourceIDToDeployState,
	)

	deployResourceOkChan <- false
}

type resourceIDToDeployState map[string]deployState

type deployState string

const (
	CANCELED           deployState = "CANCELED"
	CREATE_COMPLETE    deployState = "CREATE_COMPLETE"
	CREATE_FAILED      deployState = "CREATE_FAILED"
	CREATE_IN_PROGRESS deployState = "CREATE_IN_PROGRESS"
	DELETE_COMPLETE    deployState = "DELETE_COMPLETE"
	DELETE_FAILED      deployState = "DEPLOY_FAILED"
	DELETE_IN_PROGRESS deployState = "DELETE_IN_PROGRESS"
	PENDING            deployState = "PENDING"
	UPDATE_COMPLETE    deployState = "UPDATE_COMPLETE"
	UPDATE_FAILED      deployState = "UPDATE_FAILED"
	UPDATE_IN_PROGRESS deployState = "UPDATE_IN_PROGRESS"
)

func hasResourceIDsToDeploy(stateToResourceIDs resources.StateToResourceIDs) bool {
	statesToDeploy := []resources.State{resources.State(resources.CREATED), resources.State(resources.DELETED), resources.State(resources.UPDATED)}
	for _, state := range statesToDeploy {
		if _, exists := stateToResourceIDs[state]; exists {
			return true
		}
	}
	return false
}

func logResourceIDDeployState(group int, depth int, resourceID string, timestamp int64, resourceIDToDeployState resourceIDToDeployState) {
	date := time.Unix(0, timestamp*int64(time.Millisecond))
	hours := fmt.Sprintf("%02d", date.Hour())
	minutes := fmt.Sprintf("%02d", date.Minute())
	seconds := fmt.Sprintf("%02d", date.Second())
	formattedTime := fmt.Sprintf("%s:%s:%s", hours, minutes, seconds)

	fmt.Printf("[%s] Group %d -> Depth %d -> %s -> %s\n",
		formattedTime,
		group,
		depth,
		resourceID,
		resourceIDToDeployState[resourceID],
	)
}

func logResourceIDPreDeployStates(groupToDepthToResourceID resources.GroupToDepthToResourceIDs, resourceIDToState resources.ResourceIDToState) {
	fmt.Println("# Pre-Deploy States:")
	for group, depthToResourceID := range groupToDepthToResourceID {
		for depth, resourceIDs := range depthToResourceID {
			for _, resourceID := range resourceIDs {
				fmt.Printf("Group %d -> Depth %d -> %s -> %s\n", group, depth, resourceID, resourceIDToState[resourceID])
			}
		}
	}
}

type initialResourceIDsToDeploy []string

/*
["core:base:cloudflare-worker:12345"]

Deployments can't only start at the highest depth
containing a resource to deploy (i.e. a resource
with a deploy state of PENDING).

For example, given a graph of:
a -> b
b -> c
c -> d
a -> e

d has a depth of 3 and e has a depth of 1.

If just d and e need to be deployed, the deployment can't start
at depth 3 only. e would be blocked until d finished because
d has a higher depth than e. That's not optimal. They should
be started at the same time and deployed concurrently.
*/
func setInitialGroupResourceIDsToDeploy(highestDepthContainingAResourceToDeploy int, group int, groupToDepthToResourceIDs resources.GroupToDepthToResourceIDs, resourceIDToData resources.ResourceIDToData, resourceIDToDeployState resourceIDToDeployState) initialResourceIDsToDeploy {
	var result initialResourceIDsToDeploy

	// Add every resource at highest deploy depth containing
	// a resource to deploy.
	result = append(result, groupToDepthToResourceIDs[group][highestDepthContainingAResourceToDeploy]...)

	// Check all other depths, except 0, for resources that can
	// start deploying on deployment initiation (0 is skipped
	// because a resource at that depth can only be deployed
	// first if it's being deployed in isolation).
	depthToCheck := highestDepthContainingAResourceToDeploy - 1
	for depthToCheck > 0 {
		for _, resourceIDAtDepthToCheck := range groupToDepthToResourceIDs[group][depthToCheck] {
			for _, dependencyID := range resourceIDToData[resourceIDAtDepthToCheck].Dependencies {
				// If resource at depth to check is PENDING and is not
				// dependent on any resource in the ongoing result, then
				// append it to the result.
				if resourceIDToDeployState[resourceIDAtDepthToCheck] == deployState("PENDING") && !helpers.IsInSlice(result, dependencyID) {
					result = append(result, resourceIDAtDepthToCheck)
				}
			}
		}
		depthToCheck--
	}

	return result
}

type numOfResourcesInGroupToDeploy int

func setNumOfResourcesInGroupToDeploy(groupToResourceIDs resources.GroupToResourceIDs, resourceIDToState resources.ResourceIDToState, group int) numOfResourcesInGroupToDeploy {
	result := numOfResourcesInGroupToDeploy(0)
	for _, resourceID := range groupToResourceIDs[group] {
		if resourceIDToState[resourceID] != resources.State(resources.UNCHANGED) {
			result++
		}
	}
	return result
}

func updateResourceIDToDeployStateOfCanceled(resourceIDToDeployState resourceIDToDeployState) int {
	result := 0
	for resourceID, resourceDeployState := range resourceIDToDeployState {
		if resourceDeployState == deployState(PENDING) {
			resourceIDToDeployState[resourceID] = deployState(CANCELED)
			result++
		}
	}
	return result
}

func updateResourceIDToDeployStateOnErr(resourceIDToDeployState resourceIDToDeployState, resourceID string) {
	switch resourceIDToDeployState[resourceID] {
	case deployState(CREATE_IN_PROGRESS):
		resourceIDToDeployState[resourceID] = deployState(CREATE_FAILED)
	case deployState(DELETE_IN_PROGRESS):
		resourceIDToDeployState[resourceID] = deployState(DELETE_FAILED)
	case deployState(UPDATE_IN_PROGRESS):
		resourceIDToDeployState[resourceID] = deployState(UPDATE_FAILED)
	}
}

func updateResourceIDToDeployStateOfPending(resourceIDToState resources.ResourceIDToState) resourceIDToDeployState {
	result := make(resourceIDToDeployState)
	for resourceID, state := range resourceIDToState {
		if state != resources.State(resources.UNCHANGED) {
			result[resourceID] = deployState(PENDING)
		}
	}
	return result
}

func updateResourceIDToDeployStateOnOk(resourceIDToDeployState resourceIDToDeployState, resourceID string) {
	switch resourceIDToDeployState[resourceID] {
	case deployState(CREATE_IN_PROGRESS):
		resourceIDToDeployState[resourceID] = deployState(CREATE_COMPLETE)
	case deployState(DELETE_IN_PROGRESS):
		resourceIDToDeployState[resourceID] = deployState(DELETE_COMPLETE)
	case deployState(UPDATE_IN_PROGRESS):
		resourceIDToDeployState[resourceID] = deployState(UPDATE_COMPLETE)
	}
}

func updateResourceIDToDeployStateOnStart(resourceIDToDeployState resourceIDToDeployState, resourceIDToState resources.ResourceIDToState, resourceID string) {
	switch resourceIDToState[resourceID] {
	case resources.State(resources.CREATED):
		resourceIDToDeployState[resourceID] = deployState(CREATE_IN_PROGRESS)
	case resources.State(resources.DELETED):
		resourceIDToDeployState[resourceID] = deployState(DELETE_IN_PROGRESS)
	case resources.State(resources.UPDATED):
		resourceIDToDeployState[resourceID] = deployState(UPDATE_IN_PROGRESS)
	}
}
