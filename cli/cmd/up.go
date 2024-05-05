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

type ResourceProcessors map[string]func(resourceConfig interface{}, resourceProcessOkChan ResourceProcessorOkChan)

var resourceProcessors ResourceProcessors = make(ResourceProcessors)

func init() {
	resourceProcessors["cloudflare-kv:CREATED"] = func(config interface{}, resourceProcessorOkChan ResourceProcessorOkChan) {
		c := config.(*resources.CloudflareKVConfig)

		api, err := cloudflare.NewWithAPIToken(os.Getenv("CLOUDFLARE_API_TOKEN"))

		if err != nil {
			fmt.Println("Error:", err)
			resourceProcessorOkChan <- false
			return
		}

		title := viper.GetString("project") + "-" + helpers.CapitalSnakeCaseToTrainCase(c.Name)

		req := cloudflare.CreateWorkersKVNamespaceParams{Title: title}

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
		}

		err = validators.ValidateContainerSubdirContents(currResourceContainerSubdirPaths)
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}

		currResourceIndexFilePaths, err := resources.GetIndexFilePaths(currResourceContainerSubdirPaths)
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}

		currResourceIndexBuildFilePaths, err := resources.GetIndexBuildFilePaths(currResourceContainerSubdirPaths)
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}

		currResourceIndexBuildFileConfigs, err := resources.GetIndexBuildFileConfigs(currResourceContainerSubdirPaths, currResourceIndexFilePaths, currResourceIndexBuildFilePaths)
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}

		currResourceNameToConfig := resources.SetNameToConfig(currResourceIndexBuildFileConfigs)

		helpers.PrettyPrint(currResourceNameToConfig)

		currResourcePackageJsons, err := resources.GetPackageJsons(currResourceContainerSubdirPaths)
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}

		currResourcePackageJsonNameToTrue := resources.SetPackageJsonNameToTrue(currResourcePackageJsons)

		currResourcePackageJsonNameToResourceName := resources.SetPackageJsonNameToName(currResourcePackageJsons, currResourceIndexBuildFileConfigs)

		currResourceDependencyNames := resources.SetDependencyNames(currResourcePackageJsons, currResourcePackageJsonNameToResourceName, currResourcePackageJsonNameToTrue)

		currResourceNameToDependencies := resources.SetNameToDependencies(currResourceIndexBuildFileConfigs, currResourceDependencyNames)

		currResourceNameToData := resources.SetNameToData(currResourceIndexBuildFileConfigs, currResourceDependencyNames)

		resourceNameToInDegrees := resources.SetNameToInDegrees(currResourceNameToDependencies)

		resourceNamesWithInDegreesOfZero := resources.SetNamesWithInDegreesOf(resourceNameToInDegrees, 0)

		// TODO: Might not need this
		resourceNames := resources.SetNames(currResourceNameToData)

		resourceNameToIntermediateNames := resources.SetNameToIntermediateNames(currResourceNameToDependencies)

		resourceNameToGroup := resources.SetNameToGroup(resourceNamesWithInDegreesOfZero, resourceNameToIntermediateNames)

		depthToResourceName := resources.SetDepthToName(resourceNames, currResourceNameToDependencies, resourceNamesWithInDegreesOfZero)

		resourceNameToDepth := resources.SetNameToDepth(depthToResourceName)

		groupToDepthToResourceNames := resources.SetGroupToDepthToNames(resourceNameToGroup, resourceNameToDepth)

		// TODO: json path can be configged?
		// TODO: Or implement up -> driver -> local | gh in the config?
		resourcesUpJsonPath := "gas.up.json"

		isResourcesUpJsonPresent := helpers.IsFilePresent(resourcesUpJsonPath)

		if !isResourcesUpJsonPresent {
			err = helpers.WriteFile(resourcesUpJsonPath, "{}")
			if err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}
		}

		resourcesUpJson, err := resources.GetUpJson(resourcesUpJsonPath)
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}

		resourceNameToState := resources.SetNameToStateMap(resourcesUpJson, currResourceNameToData)

		stateToResourceNames := resources.SetStateToNames(resourceNameToState)

		hasResourceNamesToDeploy := hasResourceNamesToDeploy(stateToResourceNames)

		if !hasResourceNamesToDeploy {
			fmt.Println("No resource changes to deploy")
			os.Exit(0)
		}

		err = deploy(resourceNameToState, resourceNameToGroup, resourceNameToDepth, groupToDepthToResourceNames, currResourceNameToData)

		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}

		fmt.Println("Deployment successful")
	},
}

func deploy(resourceNameToState resources.NameToState, resourceNameToGroup resources.NameToGroup, resourceNameToDepth resources.NameToDepth, groupToDepthToResourceNames resources.GroupToDepthToNames, currResourceNameToData resources.NameToData) error {
	logResourceNamePreDeployStates(groupToDepthToResourceNames, resourceNameToState)

	resourceNameToDeployState := updateResourceNameToDeployStateOfPending(resourceNameToState)

	err := deployGroups(resourceNameToDeployState, resourceNameToState, groupToDepthToResourceNames, currResourceNameToData, resourceNameToDepth, resourceNameToGroup)

	if err != nil {
		return err
	}

	return nil
}

func deployGroups(resourceNameToDeployState resourceNameToDeployState, resourceNameToState resources.NameToState, groupToDepthToResourceNames resources.GroupToDepthToNames, currResourceNameToData resources.NameToData, resourceNameToDepth resources.NameToDepth, resourceNameToGroup resources.NameToGroup) error {
	groupsWithStateChanges := resources.SetGroupsWithStateChanges(resourceNameToGroup, resourceNameToState)

	groupsToResourceNames := resources.SetGroupToNames(resourceNameToGroup)

	groupToHighestDeployDepth := resources.SetGroupToHighestDeployDepth(
		resourceNameToDepth,
		resourceNameToState,
		groupsWithStateChanges,
		groupsToResourceNames,
	)

	numOfGroupsToDeploy := len(groupsWithStateChanges)

	deployGroupOkChan := make(DeployGroupOkChan)

	for _, group := range groupsWithStateChanges {
		go deployGroup(group, deployGroupOkChan, resourceNameToDeployState, resourceNameToState, groupToHighestDeployDepth, groupToDepthToResourceNames, currResourceNameToData, resourceNameToDepth, groupsToResourceNames)
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

func deployGroup(group int, deployGroupOkChan DeployGroupOkChan, resourceNameToDeployState resourceNameToDeployState, resourceNameToState resources.NameToState, groupToHighestDeployDepth resources.GroupToHighestDeployDepth, groupToDepthToResourceNames resources.GroupToDepthToNames, currResourceNameToData resources.NameToData, resourceNameToDepth resources.NameToDepth, groupsToResourceNames resources.GroupToNames) {

	deployResourceOkChan := make(DeployResourceOkChan)

	highestGroupDeployDepth := groupToHighestDeployDepth[group]

	initialGroupResourceNamesToDeploy := setInitialGroupResourceNamesToDeploy(highestGroupDeployDepth, group, groupToDepthToResourceNames, currResourceNameToData, resourceNameToDeployState)

	for _, resourceName := range initialGroupResourceNamesToDeploy {
		depth := resourceNameToDepth[resourceName]
		go deployResource(deployResourceOkChan, group, depth, resourceName, resourceNameToDeployState, resourceNameToState, currResourceNameToData)
	}

	numOfResourcesInGroupToDeploy := setNumOfResourcesInGroupToDeploy(
		groupsToResourceNames,
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
				numOfResourcesDeployedCanceled = updateResourceNameToDeployStateOfCanceled(resourceNameToDeployState)
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
			for _, resourceName := range groupsToResourceNames[group] {
				if resourceNameToDeployState[resourceName] == deployState("PENDING") {
					shouldDeployResource := true

					// Is resource dependent on another deploying resource?
					for _, dependencyName := range currResourceNameToData[resourceName].Dependencies {
						activeStates := map[deployState]bool{
							deployState(CREATE_IN_PROGRESS): true,
							deployState(DELETE_IN_PROGRESS): true,
							deployState(PENDING):            true,
							deployState(UPDATE_IN_PROGRESS): true,
						}

						dependencyNameDeployState := resourceNameToDeployState[dependencyName]

						if activeStates[dependencyNameDeployState] {
							shouldDeployResource = false
							break
						}
					}

					if shouldDeployResource {
						depth := resourceNameToDepth[resourceName]
						go deployResource(deployResourceOkChan, group, depth, resourceName, resourceNameToDeployState, resourceNameToState, currResourceNameToData)
					}
				}
			}
		}
	}
}

type DeployResourceOkChan chan bool

func deployResource(deployResourceOkChan DeployResourceOkChan, group int, depth int, resourceName string, resourceNameToDeployState resourceNameToDeployState, resourceNameToState resources.NameToState, currResourceNameToData resources.NameToData) {
	updateResourceNameToDeployStateOnStart(resourceNameToDeployState, resourceNameToState, resourceName)

	timestamp := time.Now().UnixMilli()

	logResourceNameDeployState(group, depth, resourceName, timestamp, resourceNameToDeployState)

	resourceProcessorOkChan := make(ResourceProcessorOkChan)

	resourceProcessorKey := currResourceNameToData[resourceName].Type + ":" + string(resourceNameToState[resourceName])

	go resourceProcessors[string(resourceProcessorKey)](currResourceNameToData[resourceName].Config, resourceProcessorOkChan)

	if <-resourceProcessorOkChan {
		updateResourceNameToDeployStateOnOk(
			resourceNameToDeployState,
			resourceName,
		)

		timestamp = time.Now().UnixMilli()

		logResourceNameDeployState(
			group,
			depth,
			resourceName,
			timestamp,
			resourceNameToDeployState,
		)

		deployResourceOkChan <- true

		return
	}

	updateResourceNameToDeployStateOnErr(
		resourceNameToDeployState,
		resourceName,
	)

	timestamp = time.Now().UnixMilli()

	logResourceNameDeployState(
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

func hasResourceNamesToDeploy(stateToResourceNames resources.StateToNames) bool {
	statesToDeploy := []resources.State{resources.State(resources.CREATED), resources.State(resources.DELETED), resources.State(resources.UPDATED)}
	for _, state := range statesToDeploy {
		if _, exists := stateToResourceNames[state]; exists {
			return true
		}
	}
	return false
}

func logResourceNameDeployState(group int, depth int, resourceName string, timestamp int64, resourceNameToDeployState resourceNameToDeployState) {
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

func logResourceNamePreDeployStates(groupToDepthToResourceName resources.GroupToDepthToNames, resourceNameToState resources.NameToState) {
	fmt.Println("# Pre-Deploy States:")
	for group, depthToResourceName := range groupToDepthToResourceName {
		for depth, resourceNames := range depthToResourceName {
			for _, resourceName := range resourceNames {
				fmt.Printf("Group %d -> Depth %d -> %s -> %s\n", group, depth, resourceName, resourceNameToState[resourceName])
			}
		}
	}
}

type initialResourceNamesToDeploy []string

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
func setInitialGroupResourceNamesToDeploy(highestDepthContainingAResourceToDeploy int, group int, groupToDepthToResourceNames resources.GroupToDepthToNames, resourceNameToData resources.NameToData, resourceNameToDeployState resourceNameToDeployState) initialResourceNamesToDeploy {
	var result initialResourceNamesToDeploy

	// Add every resource at highest deploy depth containing
	// a resource to deploy.
	result = append(result, groupToDepthToResourceNames[group][highestDepthContainingAResourceToDeploy]...)

	// Check all other depths, except 0, for resources that can
	// start deploying on deployment initiation (0 is skipped
	// because a resource at that depth can only be deployed
	// first if it's being deployed in isolation).
	depthToCheck := highestDepthContainingAResourceToDeploy - 1
	for depthToCheck > 0 {
		for _, resourceNameAtDepthToCheck := range groupToDepthToResourceNames[group][depthToCheck] {
			for _, dependencyName := range resourceNameToData[resourceNameAtDepthToCheck].Dependencies {
				// If resource at depth to check is PENDING and is not
				// dependent on any resource in the ongoing result, then
				// append it to the result.
				if resourceNameToDeployState[resourceNameAtDepthToCheck] == deployState("PENDING") && !helpers.IsInSlice(result, dependencyName) {
					result = append(result, resourceNameAtDepthToCheck)
				}
			}
		}
		depthToCheck--
	}

	return result
}

type numOfResourcesInGroupToDeploy int

func setNumOfResourcesInGroupToDeploy(groupToResourceNames resources.GroupToNames, resourceNameToState resources.NameToState, group int) numOfResourcesInGroupToDeploy {
	result := numOfResourcesInGroupToDeploy(0)
	for _, resourceName := range groupToResourceNames[group] {
		if resourceNameToState[resourceName] != resources.State(resources.UNCHANGED) {
			result++
		}
	}
	return result
}

func updateResourceNameToDeployStateOfCanceled(resourceNameToDeployState resourceNameToDeployState) int {
	result := 0
	for resourceName, resourceDeployState := range resourceNameToDeployState {
		if resourceDeployState == deployState(PENDING) {
			resourceNameToDeployState[resourceName] = deployState(CANCELED)
			result++
		}
	}
	return result
}

func updateResourceNameToDeployStateOnErr(resourceNameToDeployState resourceNameToDeployState, resourceName string) {
	switch resourceNameToDeployState[resourceName] {
	case deployState(CREATE_IN_PROGRESS):
		resourceNameToDeployState[resourceName] = deployState(CREATE_FAILED)
	case deployState(DELETE_IN_PROGRESS):
		resourceNameToDeployState[resourceName] = deployState(DELETE_FAILED)
	case deployState(UPDATE_IN_PROGRESS):
		resourceNameToDeployState[resourceName] = deployState(UPDATE_FAILED)
	}
}

func updateResourceNameToDeployStateOfPending(resourceNameToState resources.NameToState) resourceNameToDeployState {
	result := make(resourceNameToDeployState)
	for resourceName, state := range resourceNameToState {
		if state != resources.State(resources.UNCHANGED) {
			result[resourceName] = deployState(PENDING)
		}
	}
	return result
}

func updateResourceNameToDeployStateOnOk(resourceNameToDeployState resourceNameToDeployState, resourceName string) {
	switch resourceNameToDeployState[resourceName] {
	case deployState(CREATE_IN_PROGRESS):
		resourceNameToDeployState[resourceName] = deployState(CREATE_COMPLETE)
	case deployState(DELETE_IN_PROGRESS):
		resourceNameToDeployState[resourceName] = deployState(DELETE_COMPLETE)
	case deployState(UPDATE_IN_PROGRESS):
		resourceNameToDeployState[resourceName] = deployState(UPDATE_COMPLETE)
	}
}

func updateResourceNameToDeployStateOnStart(resourceNameToDeployState resourceNameToDeployState, resourceNameToState resources.NameToState, resourceName string) {
	switch resourceNameToState[resourceName] {
	case resources.State(resources.CREATED):
		resourceNameToDeployState[resourceName] = deployState(CREATE_IN_PROGRESS)
	case resources.State(resources.DELETED):
		resourceNameToDeployState[resourceName] = deployState(DELETE_IN_PROGRESS)
	case resources.State(resources.UPDATED):
		resourceNameToDeployState[resourceName] = deployState(UPDATE_IN_PROGRESS)
	}
}
