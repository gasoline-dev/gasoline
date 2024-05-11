package cmd

import (
	"context"
	"fmt"
	"gas/internal/helpers"
	"gas/internal/resources"
	"gas/internal/validators"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type resourceProcessorOkChan = chan bool

type resourceProcessorKey string

const (
	CLOUDFLARE_KV_CREATED resourceProcessorKey = "cloudflare-kv:CREATED"
)

type resourceProcessorsType map[resourceProcessorKey]func(resourceConfig interface{}, resourceProcessOkChan resourceProcessorOkChan, resourceDeployOutput *resourceDeployOutputContainer)

var resourceProcessors resourceProcessorsType = resourceProcessorsType{
	CLOUDFLARE_KV_CREATED: func(resourceConfig interface{}, resourceProcessOkChan resourceProcessorOkChan, resourceDeployOutput *resourceDeployOutputContainer) {
		c := resourceConfig.(*resources.CloudflareKVConfig)

		api, err := cloudflare.NewWithAPIToken(os.Getenv("CLOUDFLARE_API_TOKEN"))
		if err != nil {
			fmt.Println("Error:", err)
			resourceProcessOkChan <- false
			return
		}

		title := viper.GetString("project") + "-" + helpers.CapitalSnakeCaseToTrainCase(c.Name)

		req := cloudflare.CreateWorkersKVNamespaceParams{Title: title}

		response, err := api.CreateWorkersKVNamespace(context.Background(), cloudflare.AccountIdentifier(os.Getenv("CLOUDFLARE_ACCOUNT_ID")), req)

		if err != nil {
			fmt.Println("Error:", err)
			resourceProcessOkChan <- false
			return
		}

		resourceDeployOutput.set(c.Name, CLOUDFLARE_KV_CREATED, response)

		fmt.Println(response)
		resourceProcessOkChan <- true
	},
}

type CloudflareKVOutput struct {
	ID string
}

type resourceDeployOutputContainer struct {
	mu           sync.Mutex
	nameToOutput map[string]interface{}
}

func (c *resourceDeployOutputContainer) set(name string, key resourceProcessorKey, output interface{}) {
	c.mu.Lock()
	c.nameToOutput[name] = outputs[key](output)
	c.mu.Unlock()
}

var outputs = map[resourceProcessorKey]func(output interface{}) interface{}{
	CLOUDFLARE_KV_CREATED: func(output interface{}) interface{} {
		return &CloudflareKVOutput{
			ID: "TEST",
		}
	},
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

		currResourcePackageJsons, err := resources.GetPackageJsons(currResourceContainerSubdirPaths)
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}

		currResourcePackageJsonNameToTrue := resources.SetPackageJsonNameToTrue(currResourcePackageJsons)

		currResourcePackageJsonNameToResourceName := resources.SetPackageJsonNameToName(currResourcePackageJsons, currResourceIndexBuildFileConfigs)

		currResourceDependencyNames := resources.SetDependencyNames(currResourcePackageJsons, currResourcePackageJsonNameToResourceName, currResourcePackageJsonNameToTrue)

		currResourceNameToDependencies := resources.SetNameToDependencies(currResourceIndexBuildFileConfigs, currResourceDependencyNames)

		resourceNameToInDegrees := resources.SetNameToInDegrees(currResourceNameToDependencies)

		resourceNamesWithInDegreesOfZero := resources.SetNamesWithInDegreesOf(resourceNameToInDegrees, 0)

		resourceNameToIntermediateNames := resources.SetNameToIntermediateNames(currResourceNameToDependencies)

		resourceNameToGroup := resources.SetNameToGroup(resourceNamesWithInDegreesOfZero, resourceNameToIntermediateNames)

		depthToResourceName := resources.SetDepthToName(currResourceNameToDependencies, resourceNamesWithInDegreesOfZero)

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

		upResourceNameToDependencies := resources.SetUpNameToDependencies(resourcesUpJson)

		upResourceNameToConfig := resources.SetUpNameToConfig(resourcesUpJson)

		resourceNameToState := resources.SetNameToStateMap(upResourceNameToConfig, currResourceNameToConfig, upResourceNameToDependencies, currResourceNameToDependencies)

		stateToResourceNames := resources.SetStateToNames(resourceNameToState)

		hasResourceNamesToDeploy := hasResourceNamesToDeploy(stateToResourceNames)

		if !hasResourceNamesToDeploy {
			fmt.Println("No resource changes to deploy")
			os.Exit(0)
		}

		err = deploy(
			currResourceNameToConfig,
			currResourceNameToDependencies,
			groupToDepthToResourceNames,
			resourceNameToDepth,
			resourceNameToGroup,
			resourceNameToState,
		)

		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}

		fmt.Println("Deployment successful")
	},
}

func deploy(
	currResourceNameToConfig resources.NameToConfig,
	currResourceNameToDependencies resources.NameToDependencies,
	groupToDepthToResourceNames resources.GroupToDepthToNames,
	resourceNameToDepth resources.NameToDepth,
	resourceNameToGroup resources.NameToGroup,
	resourceNameToState resources.NameToState,
) error {
	logResourceNamePreDeployStates(groupToDepthToResourceNames, resourceNameToState)

	resourceDeployState := &resourceDeployStateContainer{
		nameToDeployState: make(map[string]deployState),
	}

	resourceDeployState.setPending(resourceNameToState)

	resourceDeployOutput := &resourceDeployOutputContainer{
		nameToOutput: make(map[string]interface{}),
	}

	err := deployGroups(
		currResourceNameToConfig,
		currResourceNameToDependencies,
		groupToDepthToResourceNames,
		resourceDeployOutput,
		resourceDeployState,
		resourceNameToDepth,
		resourceNameToGroup,
		resourceNameToState,
	)

	if err != nil {
		return err
	}

	// TODO: handle gas.up.json
	fmt.Println(resourceDeployOutput.nameToOutput["CORE_BASE_KV"])

	return nil
}

func deployGroups(
	currResourceNameToConfig resources.NameToConfig,
	currResourceNameToDependencies resources.NameToDependencies,
	groupToDepthToResourceNames resources.GroupToDepthToNames,
	resourceDeployOutput *resourceDeployOutputContainer,
	resourceDeployState *resourceDeployStateContainer,
	resourceNameToDepth resources.NameToDepth,
	resourceNameToGroup resources.NameToGroup,
	resourceNameToState resources.NameToState,
) error {
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
		go deployGroup(
			currResourceNameToConfig,
			currResourceNameToDependencies,
			deployGroupOkChan,
			group,
			groupToDepthToResourceNames,
			groupToHighestDeployDepth,
			groupsToResourceNames,
			resourceDeployOutput,
			resourceDeployState,
			resourceNameToDepth,
			resourceNameToState,
		)
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

func deployGroup(
	currResourceNameToConfig resources.NameToConfig,
	currResourceNameToDependencies resources.NameToDependencies,
	deployGroupOkChan DeployGroupOkChan,
	group int,
	groupToDepthToResourceNames resources.GroupToDepthToNames,
	groupToHighestDeployDepth resources.GroupToHighestDeployDepth,
	groupsToResourceNames resources.GroupToNames,
	resourceDeployOutput *resourceDeployOutputContainer,
	resourceDeployState *resourceDeployStateContainer,
	resourceNameToDepth resources.NameToDepth,
	resourceNameToState resources.NameToState,
) {

	deployResourceOkChan := make(DeployResourceOkChan)

	highestGroupDeployDepth := groupToHighestDeployDepth[group]

	initialGroupResourceNamesToDeploy := setInitialGroupResourceNamesToDeploy(highestGroupDeployDepth, group, groupToDepthToResourceNames, resourceDeployState, currResourceNameToDependencies)

	for _, resourceName := range initialGroupResourceNamesToDeploy {
		depth := resourceNameToDepth[resourceName]
		go deployResource(
			currResourceNameToConfig,
			depth,
			deployResourceOkChan,
			group,
			resourceDeployOutput,
			resourceDeployState,
			resourceName,
			resourceNameToState,
		)
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
				numOfResourcesDeployedCanceled = resourceDeployState.setPendingToCanceled()
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
				if resourceDeployState.nameToDeployState[resourceName] == deployState("PENDING") {
					shouldDeployResource := true

					// Is resource dependent on another deploying resource?
					for _, dependencyName := range currResourceNameToDependencies[resourceName] {
						activeStates := map[deployState]bool{
							deployState(CREATE_IN_PROGRESS): true,
							deployState(DELETE_IN_PROGRESS): true,
							deployState(PENDING):            true,
							deployState(UPDATE_IN_PROGRESS): true,
						}

						dependencyNameDeployState := resourceDeployState.nameToDeployState[dependencyName]

						if activeStates[dependencyNameDeployState] {
							shouldDeployResource = false
							break
						}
					}

					if shouldDeployResource {
						depth := resourceNameToDepth[resourceName]
						go deployResource(
							currResourceNameToConfig,
							depth,
							deployResourceOkChan,
							group,
							resourceDeployOutput,
							resourceDeployState,
							resourceName,
							resourceNameToState,
						)
					}
				}
			}
		}
	}
}

type DeployResourceOkChan chan bool

func deployResource(
	currResourceNameToConfig resources.NameToConfig,
	depth int,
	deployResourceOkChan DeployResourceOkChan,
	group int,
	resourceDeployOutput *resourceDeployOutputContainer,
	resourceDeployState *resourceDeployStateContainer,
	resourceName string,
	resourceNameToState resources.NameToState,
) {
	resourceDeployState.setInProgress(resourceName, resourceNameToState)

	timestamp := time.Now().UnixMilli()

	resourceDeployState.log(
		group,
		depth,
		resourceName,
		timestamp,
	)

	resourceProcessorOkChan := make(resourceProcessorOkChan)

	resourceType := reflect.ValueOf(currResourceNameToConfig[resourceName]).Elem().FieldByName("Type").String()

	resourceProcessorKey := resourceProcessorKey(resourceType + ":" + string(resourceNameToState[resourceName]))

	go resourceProcessors[resourceProcessorKey](currResourceNameToConfig[resourceName], resourceProcessorOkChan, resourceDeployOutput)

	if <-resourceProcessorOkChan {
		resourceDeployState.setComplete(resourceName)

		timestamp = time.Now().UnixMilli()

		resourceDeployState.log(
			group,
			depth,
			resourceName,
			timestamp,
		)

		deployResourceOkChan <- true

		return
	}

	resourceDeployState.setFailed(resourceName)

	timestamp = time.Now().UnixMilli()

	resourceDeployState.log(
		group,
		depth,
		resourceName,
		timestamp,
	)

	deployResourceOkChan <- false
}

type resourceDeployStateContainer struct {
	mu                sync.Mutex
	nameToDeployState map[string]deployState
}

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

func (c *resourceDeployStateContainer) log(group int, depth int, name string, timestamp int64) {
	date := time.Unix(0, timestamp*int64(time.Millisecond))
	hours := fmt.Sprintf("%02d", date.Hour())
	minutes := fmt.Sprintf("%02d", date.Minute())
	seconds := fmt.Sprintf("%02d", date.Second())
	formattedTime := fmt.Sprintf("%s:%s:%s", hours, minutes, seconds)

	fmt.Printf("[%s] Group %d -> Depth %d -> %s -> %s\n",
		formattedTime,
		group,
		depth,
		name,
		c.nameToDeployState[name],
	)
}

func (c *resourceDeployStateContainer) setComplete(name string) {
	c.mu.Lock()
	switch c.nameToDeployState[name] {
	case deployState(CREATE_IN_PROGRESS):
		c.nameToDeployState[name] = deployState(CREATE_COMPLETE)
	case deployState(DELETE_IN_PROGRESS):
		c.nameToDeployState[name] = deployState(DELETE_COMPLETE)
	case deployState(UPDATE_IN_PROGRESS):
		c.nameToDeployState[name] = deployState(UPDATE_COMPLETE)
	}
	c.mu.Unlock()
}

func (c *resourceDeployStateContainer) setFailed(name string) {
	c.mu.Lock()
	switch c.nameToDeployState[name] {
	case deployState(CREATE_IN_PROGRESS):
		c.nameToDeployState[name] = deployState(CREATE_FAILED)
	case deployState(DELETE_IN_PROGRESS):
		c.nameToDeployState[name] = deployState(DELETE_FAILED)
	case deployState(UPDATE_IN_PROGRESS):
		c.nameToDeployState[name] = deployState(UPDATE_FAILED)
	}
	c.mu.Unlock()
}

func (c *resourceDeployStateContainer) setInProgress(resourceName string, resourceNameToState resources.NameToState) {
	c.mu.Lock()
	switch resourceNameToState[resourceName] {
	case resources.State(resources.CREATED):
		c.nameToDeployState[resourceName] = deployState(CREATE_IN_PROGRESS)
	case resources.State(resources.DELETED):
		c.nameToDeployState[resourceName] = deployState(DELETE_IN_PROGRESS)
	case resources.State(resources.UPDATED):
		c.nameToDeployState[resourceName] = deployState(UPDATE_IN_PROGRESS)
	}
	c.mu.Unlock()
}

func (c *resourceDeployStateContainer) setPending(nameToState resources.NameToState) {
	c.mu.Lock()
	for name, state := range nameToState {
		if state != resources.State(resources.UNCHANGED) {
			c.nameToDeployState[name] = deployState(PENDING)
		}
	}
	c.mu.Unlock()
}

func (c *resourceDeployStateContainer) setPendingToCanceled() int {
	c.mu.Lock()
	result := 0
	for name, state := range c.nameToDeployState {
		if state == deployState(PENDING) {
			c.nameToDeployState[name] = deployState(CANCELED)
		}
	}
	c.mu.Unlock()
	return result
}

func hasResourceNamesToDeploy(stateToResourceNames resources.StateToNames) bool {
	statesToDeploy := []resources.State{resources.State(resources.CREATED), resources.State(resources.DELETED), resources.State(resources.UPDATED)}
	for _, state := range statesToDeploy {
		if _, exists := stateToResourceNames[state]; exists {
			return true
		}
	}
	return false
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
func setInitialGroupResourceNamesToDeploy(
	highestDepthContainingAResourceToDeploy int,
	group int,
	groupToDepthToResourceNames resources.GroupToDepthToNames,
	resourceDeployState *resourceDeployStateContainer,
	resourceNameToDependencies resources.NameToDependencies,
) initialResourceNamesToDeploy {
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
			for _, dependencyName := range resourceNameToDependencies[resourceNameAtDepthToCheck] {
				// If resource at depth to check is PENDING and is not
				// dependent on any resource in the ongoing result, then
				// append it to the result.
				if resourceDeployState.nameToDeployState[resourceNameAtDepthToCheck] == deployState("PENDING") && !helpers.IsInSlice(result, dependencyName) {
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
