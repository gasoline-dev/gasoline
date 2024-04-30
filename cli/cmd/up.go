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

type ResourceProcessors map[string]func(resourceConfig any, resourceProcessOkChan ResourceProcessorOkChan)

var resourceProcessors ResourceProcessors = make(ResourceProcessors)

func init() {
	resourceProcessors["cloudflare-kv:CREATED"] = func(arg interface{}, resourceProcessorOkChan ResourceProcessorOkChan) {
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

var upCmd = &cobra.Command{
	Use:   "up",
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

		currResourceIndexFilePaths, err := resources.GetIndexFilePaths(currResourceContainerSubdirPaths)
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

		currResourceIndexBuildFileConfigs, err := resources.GetIndexBuildFileConfigs(currResourceContainerSubdirPaths, currResourceIndexFilePaths, currResourceIndexBuildFilePaths)
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

		currResourcePackageJsonNameToID := resources.SetPackageJsonNameToResourceName(currResourcePackageJsons, currResourceIndexBuildFileConfigs)

		currResourceDependencyIDs := resources.SetDependencyNames(currResourcePackageJsons, currResourcePackageJsonNameToID, currResourcePackageJsonNameToTrue)

		currResourceIDToData := resources.SetNameToData(currResourceIndexBuildFileConfigs, currResourceDependencyIDs)

		resourceNameToInDegrees := resources.SetNameToInDegrees(currResourceIDToData)

		resourceNamesWithInDegreesOfZero := resources.SetNamesWithInDegreesOf(resourceNameToInDegrees, 0)

		// TODO: Might not need this
		resourceNames := resources.SetNames(currResourceIDToData)

		resourceNameToIntermediateIDs := resources.SetNameToIntermediateNames(currResourceIDToData)

		resourceNameToGroup := resources.SetNameToGroup(resourceNamesWithInDegreesOfZero, resourceNameToIntermediateIDs)

		depthToResourceID := resources.SetDepthToResourceName(resourceNames, currResourceIDToData, resourceNamesWithInDegreesOfZero)

		resourceNameToDepth := resources.SetNameToDepth(depthToResourceID)

		groupToDepthToResourceIDs := resources.SetGroupToDepthToResourceNames(resourceNameToGroup, resourceNameToDepth)

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

		resourceNameToState := resources.SetNameToStateMap(resourcesUpJson, currResourceIDToData)

		stateToResourceIDs := resources.SetStateToResourceNames(resourceNameToState)

		hasResourceIDsToDeploy := hasResourceIDsToDeploy(stateToResourceIDs)

		if !hasResourceIDsToDeploy {
			fmt.Println("No resource changes to deploy")
			return
		}

		err = deploy(resourceNameToState, resourceNameToGroup, resourceNameToDepth, groupToDepthToResourceIDs, currResourceIDToData)

		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
			return
		}

		fmt.Println("Deployment successful")
	},
}

func deploy(resourceNameToState resources.ResourceNameToState, resourceNameToGroup resources.ResourceNameToGroup, resourceNameToDepth resources.ResourceNameToDepth, groupToDepthToResourceIDs resources.GroupToDepthToResourceNames, currResourceIDToData resources.ResourceNameToData) error {
	logResourceIDPreDeployStates(groupToDepthToResourceIDs, resourceNameToState)

	resourceNameToDeployState := updateResourceIDToDeployStateOfPending(resourceNameToState)

	err := deployGroups(resourceNameToDeployState, resourceNameToState, groupToDepthToResourceIDs, currResourceIDToData, resourceNameToDepth, resourceNameToGroup)

	if err != nil {
		return err
	}

	return nil
}

func deployGroups(resourceNameToDeployState resourceNameToDeployState, resourceNameToState resources.ResourceNameToState, groupToDepthToResourceIDs resources.GroupToDepthToResourceNames, currResourceIDToData resources.ResourceNameToData, resourceNameToDepth resources.ResourceNameToDepth, resourceNameToGroup resources.ResourceNameToGroup) error {
	groupsWithStateChanges := resources.SetGroupsWithStateChanges(resourceNameToGroup, resourceNameToState)

	groupsToResourceIDs := resources.SetGroupToResourceNames(resourceNameToGroup)

	groupToHighestDeployDepth := resources.SetGroupToHighestDeployDepth(
		resourceNameToDepth,
		resourceNameToState,
		groupsWithStateChanges,
		groupsToResourceIDs,
	)

	numOfGroupsToDeploy := len(groupsWithStateChanges)

	deployGroupOkChan := make(DeployGroupOkChan)

	for _, group := range groupsWithStateChanges {
		go deployGroup(group, deployGroupOkChan, resourceNameToDeployState, resourceNameToState, groupToHighestDeployDepth, groupToDepthToResourceIDs, currResourceIDToData, resourceNameToDepth, groupsToResourceIDs)
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

func deployGroup(group int, deployGroupOkChan DeployGroupOkChan, resourceNameToDeployState resourceNameToDeployState, resourceNameToState resources.ResourceNameToState, groupToHighestDeployDepth resources.GroupToHighestDeployDepth, groupToDepthToResourceIDs resources.GroupToDepthToResourceNames, currResourceIDToData resources.ResourceNameToData, resourceNameToDepth resources.ResourceNameToDepth, groupsToResourceIDs resources.GroupToResourceNames) {

	deployResourceOkChan := make(DeployResourceOkChan)

	highestGroupDeployDepth := groupToHighestDeployDepth[group]

	initialGroupResourceIDsToDeploy := setInitialGroupResourceIDsToDeploy(highestGroupDeployDepth, group, groupToDepthToResourceIDs, currResourceIDToData, resourceNameToDeployState)

	for _, resourceName := range initialGroupResourceIDsToDeploy {
		depth := resourceNameToDepth[resourceName]
		go deployResource(deployResourceOkChan, group, depth, resourceName, resourceNameToDeployState, resourceNameToState, currResourceIDToData)
	}

	numOfResourcesInGroupToDeploy := setNumOfResourcesInGroupToDeploy(
		groupsToResourceIDs,
		resourceNameToState,
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
				numOfResourcesDeployedCanceled = updateResourceIDToDeployStateOfCanceled(resourceNameToDeployState)
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
			for _, resourceName := range groupsToResourceIDs[group] {
				if resourceNameToDeployState[resourceName] == deployState("PENDING") {
					shouldDeployResource := true

					// Is resource dependent on another deploying resource?
					for _, dependencyID := range currResourceIDToData[resourceName].Dependencies {
						activeStates := map[deployState]bool{
							deployState(CREATE_IN_PROGRESS): true,
							deployState(DELETE_IN_PROGRESS): true,
							deployState(PENDING):            true,
							deployState(UPDATE_IN_PROGRESS): true,
						}

						dependencyIDDeployState := resourceNameToDeployState[dependencyID]

						if activeStates[dependencyIDDeployState] {
							shouldDeployResource = false
							break
						}
					}

					if shouldDeployResource {
						depth := resourceNameToDepth[resourceName]
						go deployResource(deployResourceOkChan, group, depth, resourceName, resourceNameToDeployState, resourceNameToState, currResourceIDToData)
					}
				}
			}
		}
	}
}

type DeployResourceOkChan chan bool

func deployResource(deployResourceOkChan DeployResourceOkChan, group int, depth int, resourceName string, resourceNameToDeployState resourceNameToDeployState, resourceNameToState resources.ResourceNameToState, currResourceIDToData resources.ResourceNameToData) {
	updateResourceIDToDeployStateOnStart(resourceNameToDeployState, resourceNameToState, resourceName)

	timestamp := time.Now().UnixMilli()

	logResourceIDDeployState(group, depth, resourceName, timestamp, resourceNameToDeployState)

	resourceProcessorOkChan := make(ResourceProcessorOkChan)

	resourceProcessorKey := currResourceIDToData[resourceName].Type + ":" + string(resourceNameToState[resourceName])

	go resourceProcessors[string(resourceProcessorKey)](currResourceIDToData[resourceName].Config, resourceProcessorOkChan)

	if <-resourceProcessorOkChan {
		updateResourceIDToDeployStateOnOk(
			resourceNameToDeployState,
			resourceName,
		)

		timestamp = time.Now().UnixMilli()

		logResourceIDDeployState(
			group,
			depth,
			resourceName,
			timestamp,
			resourceNameToDeployState,
		)

		deployResourceOkChan <- true

		return
	}

	updateResourceIDToDeployStateOnErr(
		resourceNameToDeployState,
		resourceName,
	)

	timestamp = time.Now().UnixMilli()

	logResourceIDDeployState(
		group,
		depth,
		resourceName,
		timestamp,
		resourceNameToDeployState,
	)

	deployResourceOkChan <- false
}

type resourceNameToDeployState map[string]deployState

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

func hasResourceIDsToDeploy(stateToResourceIDs resources.StateToResourceNames) bool {
	statesToDeploy := []resources.State{resources.State(resources.CREATED), resources.State(resources.DELETED), resources.State(resources.UPDATED)}
	for _, state := range statesToDeploy {
		if _, exists := stateToResourceIDs[state]; exists {
			return true
		}
	}
	return false
}

func logResourceIDDeployState(group int, depth int, resourceName string, timestamp int64, resourceNameToDeployState resourceNameToDeployState) {
	date := time.Unix(0, timestamp*int64(time.Millisecond))
	hours := fmt.Sprintf("%02d", date.Hour())
	minutes := fmt.Sprintf("%02d", date.Minute())
	seconds := fmt.Sprintf("%02d", date.Second())
	formattedTime := fmt.Sprintf("%s:%s:%s", hours, minutes, seconds)

	fmt.Printf("[%s] Group %d -> Depth %d -> %s -> %s\n",
		formattedTime,
		group,
		depth,
		resourceName,
		resourceNameToDeployState[resourceName],
	)
}

func logResourceIDPreDeployStates(groupToDepthToResourceID resources.GroupToDepthToResourceNames, resourceNameToState resources.ResourceNameToState) {
	fmt.Println("# Pre-Deploy States:")
	for group, depthToResourceID := range groupToDepthToResourceID {
		for depth, resourceNames := range depthToResourceID {
			for _, resourceName := range resourceNames {
				fmt.Printf("Group %d -> Depth %d -> %s -> %s\n", group, depth, resourceName, resourceNameToState[resourceName])
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
func setInitialGroupResourceIDsToDeploy(highestDepthContainingAResourceToDeploy int, group int, groupToDepthToResourceIDs resources.GroupToDepthToResourceNames, resourceNameToData resources.ResourceNameToData, resourceNameToDeployState resourceNameToDeployState) initialResourceIDsToDeploy {
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
		for _, resourceNameAtDepthToCheck := range groupToDepthToResourceIDs[group][depthToCheck] {
			for _, dependencyID := range resourceNameToData[resourceNameAtDepthToCheck].Dependencies {
				// If resource at depth to check is PENDING and is not
				// dependent on any resource in the ongoing result, then
				// append it to the result.
				if resourceNameToDeployState[resourceNameAtDepthToCheck] == deployState("PENDING") && !helpers.IsInSlice(result, dependencyID) {
					result = append(result, resourceNameAtDepthToCheck)
				}
			}
		}
		depthToCheck--
	}

	return result
}

type numOfResourcesInGroupToDeploy int

func setNumOfResourcesInGroupToDeploy(groupToResourceIDs resources.GroupToResourceNames, resourceNameToState resources.ResourceNameToState, group int) numOfResourcesInGroupToDeploy {
	result := numOfResourcesInGroupToDeploy(0)
	for _, resourceName := range groupToResourceIDs[group] {
		if resourceNameToState[resourceName] != resources.State(resources.UNCHANGED) {
			result++
		}
	}
	return result
}

func updateResourceIDToDeployStateOfCanceled(resourceNameToDeployState resourceNameToDeployState) int {
	result := 0
	for resourceName, resourceDeployState := range resourceNameToDeployState {
		if resourceDeployState == deployState(PENDING) {
			resourceNameToDeployState[resourceName] = deployState(CANCELED)
			result++
		}
	}
	return result
}

func updateResourceIDToDeployStateOnErr(resourceNameToDeployState resourceNameToDeployState, resourceName string) {
	switch resourceNameToDeployState[resourceName] {
	case deployState(CREATE_IN_PROGRESS):
		resourceNameToDeployState[resourceName] = deployState(CREATE_FAILED)
	case deployState(DELETE_IN_PROGRESS):
		resourceNameToDeployState[resourceName] = deployState(DELETE_FAILED)
	case deployState(UPDATE_IN_PROGRESS):
		resourceNameToDeployState[resourceName] = deployState(UPDATE_FAILED)
	}
}

func updateResourceIDToDeployStateOfPending(resourceNameToState resources.ResourceNameToState) resourceNameToDeployState {
	result := make(resourceNameToDeployState)
	for resourceName, state := range resourceNameToState {
		if state != resources.State(resources.UNCHANGED) {
			result[resourceName] = deployState(PENDING)
		}
	}
	return result
}

func updateResourceIDToDeployStateOnOk(resourceNameToDeployState resourceNameToDeployState, resourceName string) {
	switch resourceNameToDeployState[resourceName] {
	case deployState(CREATE_IN_PROGRESS):
		resourceNameToDeployState[resourceName] = deployState(CREATE_COMPLETE)
	case deployState(DELETE_IN_PROGRESS):
		resourceNameToDeployState[resourceName] = deployState(DELETE_COMPLETE)
	case deployState(UPDATE_IN_PROGRESS):
		resourceNameToDeployState[resourceName] = deployState(UPDATE_COMPLETE)
	}
}

func updateResourceIDToDeployStateOnStart(resourceNameToDeployState resourceNameToDeployState, resourceNameToState resources.ResourceNameToState, resourceName string) {
	switch resourceNameToState[resourceName] {
	case resources.State(resources.CREATED):
		resourceNameToDeployState[resourceName] = deployState(CREATE_IN_PROGRESS)
	case resources.State(resources.DELETED):
		resourceNameToDeployState[resourceName] = deployState(DELETE_IN_PROGRESS)
	case resources.State(resources.UPDATED):
		resourceNameToDeployState[resourceName] = deployState(UPDATE_IN_PROGRESS)
	}
}
