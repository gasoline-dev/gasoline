package cmd

import (
	"encoding/json"
	"fmt"
	"gas/internal/helpers"
	"gas/internal/resources"
	"gas/internal/validators"
	"os"
	"reflect"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Deploy resources to the cloud",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Deploying resources to the cloud")

		resourceContainerDir := viper.GetString("resourceContainerDirPath")

		currResourceContainerSubdirPaths, err := resources.GetContainerSubdirPaths(resourceContainerDir)
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}

		err = validators.ValidateContainerSubdirContents(currResourceContainerSubdirPaths)
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}

		var currResourceNameToConfig resources.NameToConfig
		var currResourceNameToDependencies resources.NameToDependencies

		if len(currResourceContainerSubdirPaths) > 0 {
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

			if len(currResourceIndexBuildFilePaths) > 0 {
				currResourceIndexBuildFileConfigs, err := resources.GetIndexBuildFileConfigs(
					currResourceContainerSubdirPaths,
					currResourceIndexFilePaths,
					currResourceIndexBuildFilePaths,
				)
				if err != nil {
					fmt.Println("Error:", err)
					os.Exit(1)
				}

				currResourceNameToConfig = resources.SetNameToConfig(currResourceIndexBuildFileConfigs)

				currResourcePackageJsons, err := resources.GetPackageJsons(currResourceContainerSubdirPaths)
				if err != nil {
					fmt.Println("Error:", err)
					os.Exit(1)
				}

				currResourcePackageJsonNameToTrue := resources.SetPackageJsonNameToTrue(currResourcePackageJsons)

				currResourcePackageJsonNameToResourceName := resources.SetPackageJsonNameToName(
					currResourcePackageJsons,
					currResourceIndexBuildFileConfigs,
				)

				currResourceDependencyNames := resources.SetDependencyNames(
					currResourcePackageJsons,
					currResourcePackageJsonNameToResourceName,
					currResourcePackageJsonNameToTrue,
				)

				currResourceNameToDependencies = resources.SetNameToDependencies(
					currResourceIndexBuildFileConfigs,
					currResourceDependencyNames,
				)
			}
		}

		upJsonPath := viper.GetString("upJsonPath")

		upJson, err := resources.GetUpJson(upJsonPath)
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}

		var upResourceNameToConfig resources.UpNameToConfig
		var upResourceNameToDependencies resources.UpNameToDependencies

		if len(upJson) > 0 {
			upResourceNameToConfig = resources.SetUpNameToConfig(upJson)

			upResourceNameToDependencies = resources.SetUpNameToDependencies(upJson)
		}

		if len(upResourceNameToConfig) == 0 && len(currResourceNameToConfig) == 0 && len(upResourceNameToDependencies) == 0 && len(currResourceNameToDependencies) == 0 {
			fmt.Printf(
				"No resources found in %s or %s",
				resourceContainerDir,
				upJsonPath,
			)
			os.Exit(0)
		}

		resourceNameToState := resources.SetNameToState(
			upResourceNameToConfig,
			currResourceNameToConfig,
			upResourceNameToDependencies,
			currResourceNameToDependencies,
		)

		hasResourceNamesToDeploy := checkIfResourceNamesToDeploy(resourceNameToState)

		if !hasResourceNamesToDeploy {
			fmt.Println("No resource changes to deploy")
			os.Exit(0)
		}

		/*
			up json may have resources that don't exist in
			curr resource maps because the resources were deleted.
			Those resources are accounted for by merging up and
			curr maps. Only then are the graphs complete.
		*/

		var resourceNameToConfig resources.NameToConfig

		if len(upResourceNameToConfig) > 0 || len(currResourceNameToConfig) > 0 {
			resourceNameToConfig = resources.NameToConfig(helpers.MergeInterfaceMaps(upResourceNameToConfig, currResourceNameToConfig))
		}

		var resourceNameToDependencies resources.NameToDependencies

		if len(upResourceNameToDependencies) > 0 || len(currResourceNameToDependencies) > 0 {
			resourceNameToDependencies = resources.NameToDependencies(helpers.MergeStringSliceMaps(upResourceNameToDependencies, currResourceNameToDependencies))
		}

		resourceNameToInDegrees := resources.SetNameToInDegrees(resourceNameToDependencies)

		resourceNamesWithInDegreesOfZero := resources.SetNamesWithInDegreesOf(resourceNameToInDegrees, 0)

		resourceNameToIntermediateNames := resources.SetNameToIntermediateNames(resourceNameToDependencies)

		resourceNameToGroup := resources.SetNameToGroup(resourceNamesWithInDegreesOfZero, resourceNameToIntermediateNames)

		depthToResourceName := resources.SetDepthToName(resourceNameToDependencies, resourceNamesWithInDegreesOfZero)

		resourceNameToDepth := resources.SetNameToDepth(depthToResourceName)

		groupToDepthToResourceNames := resources.SetGroupToDepthToNames(resourceNameToGroup, resourceNameToDepth)

		err = deploy(
			resourceNameToConfig,
			resourceNameToDependencies,
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

	resourceNameToDeployState := &resources.NameToDeployStateContainer{
		M: make(map[string]resources.DeployState),
	}

	resourceNameToDeployState.SetPending(resourceNameToState)

	resourceNameToDeployOutput := &resources.NameToDeployOutputContainer{
		M: make(map[string]interface{}),
	}

	err := deployGroups(
		currResourceNameToConfig,
		currResourceNameToDependencies,
		groupToDepthToResourceNames,
		resourceNameToDeployOutput,
		resourceNameToDeployState,
		resourceNameToDepth,
		resourceNameToGroup,
		resourceNameToState,
	)

	if err != nil {
		return err
	}

	newUpjson := make(resources.UpJson)

	for resourceName, output := range resourceNameToDeployOutput.M {
		newUpjson[resourceName] = struct {
			Config       interface{} `json:"config"`
			Dependencies []string    `json:"dependencies"`
			Output       interface{} `json:"output"`
		}{
			Config:       currResourceNameToConfig[resourceName],
			Dependencies: currResourceNameToDependencies[resourceName],
			Output:       output,
		}
	}

	jsonData, err := json.MarshalIndent(newUpjson, "", "  ")
	if err != nil {
		fmt.Println("error marshalling to JSON:", err)
		return nil
	}

	fileName := "gas.up.json"
	err = os.WriteFile(fileName, jsonData, 0644)
	if err != nil {
		fmt.Println("error writing to file:", err)
		return nil
	}

	return nil
}

func deployGroups(
	currResourceNameToConfig resources.NameToConfig,
	currResourceNameToDependencies resources.NameToDependencies,
	groupToDepthToResourceNames resources.GroupToDepthToNames,
	resourceNameToDeployOutput *resources.NameToDeployOutputContainer,
	resourceNameToDeployState *resources.NameToDeployStateContainer,
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
			resourceNameToDeployOutput,
			resourceNameToDeployState,
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
	resourceNameToDeployOutput *resources.NameToDeployOutputContainer,
	resourceNameToDeployState *resources.NameToDeployStateContainer,
	resourceNameToDepth resources.NameToDepth,
	resourceNameToState resources.NameToState,
) {

	deployResourceOkChan := make(DeployResourceOkChan)

	highestGroupDeployDepth := groupToHighestDeployDepth[group]

	initialGroupResourceNamesToDeploy := setInitialGroupResourceNamesToDeploy(
		highestGroupDeployDepth,
		group,
		groupToDepthToResourceNames,
		resourceNameToDeployState,
		currResourceNameToDependencies,
	)

	for _, resourceName := range initialGroupResourceNamesToDeploy {
		depth := resourceNameToDepth[resourceName]
		go deployResource(
			currResourceNameToConfig,
			depth,
			deployResourceOkChan,
			group,
			resourceNameToDeployOutput,
			resourceNameToDeployState,
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
				numOfResourcesDeployedCanceled = resourceNameToDeployState.SetPendingToCanceled()
			}
		}

		numOfResourcesInFinalDeployState := numOfResourcesDeployedOk +
			numOfResourcesDeployedErr +
			numOfResourcesDeployedCanceled

		if numOfResourcesInFinalDeployState == int(numOfResourcesInGroupToDeploy) {
			if numOfResourcesDeployedErr == 0 {
				deployGroupOkChan <- true
			} else {
				deployGroupOkChan <- false
			}
			return
		} else {
			for _, resourceName := range groupsToResourceNames[group] {
				if resourceNameToDeployState.M[resourceName] == resources.DeployState("PENDING") {
					shouldDeployResource := true

					// Is resource dependent on another deploying resource?
					for _, dependencyName := range currResourceNameToDependencies[resourceName] {
						activeStates := map[resources.DeployState]bool{
							resources.DeployState(resources.CREATE_IN_PROGRESS): true,
							resources.DeployState(resources.DELETE_IN_PROGRESS): true,
							resources.DeployState(resources.PENDING):            true,
							resources.DeployState(resources.UPDATE_IN_PROGRESS): true,
						}

						dependencyNameDeployState := resourceNameToDeployState.M[dependencyName]

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
							resourceNameToDeployOutput,
							resourceNameToDeployState,
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
	resourceNameToDeployOutput *resources.NameToDeployOutputContainer,
	resourceNameToDeployState *resources.NameToDeployStateContainer,
	resourceName string,
	resourceNameToState resources.NameToState,
) {
	resourceNameToDeployState.SetInProgress(resourceName, resourceNameToState)

	timestamp := time.Now().UnixMilli()

	resourceNameToDeployState.Log(
		group,
		depth,
		resourceName,
		timestamp,
	)

	resourceProcessorOkChan := make(resources.ProcessorOkChan)

	resourceType := reflect.ValueOf(currResourceNameToConfig[resourceName]).Elem().FieldByName("Type").String()

	resourceProcessorKey := resources.ProcessorKey(resourceType + ":" + string(resourceNameToState[resourceName]))

	go resources.Processors[resourceProcessorKey](currResourceNameToConfig[resourceName], resourceProcessorOkChan, resourceNameToDeployOutput)

	if <-resourceProcessorOkChan {
		resourceNameToDeployState.SetComplete(resourceName)

		timestamp = time.Now().UnixMilli()

		resourceNameToDeployState.Log(
			group,
			depth,
			resourceName,
			timestamp,
		)

		deployResourceOkChan <- true

		return
	}

	resourceNameToDeployState.SetFailed(resourceName)

	timestamp = time.Now().UnixMilli()

	resourceNameToDeployState.Log(
		group,
		depth,
		resourceName,
		timestamp,
	)

	deployResourceOkChan <- false
}

func checkIfResourceNamesToDeploy(nameToState resources.NameToState) bool {
	for name := range nameToState {
		if nameToState[name] != resources.State(resources.UNCHANGED) {
			return true
		}
	}
	return false
}

func logResourceNamePreDeployStates(
	groupToDepthToResourceName resources.GroupToDepthToNames,
	resourceNameToState resources.NameToState,
) {
	fmt.Println("# Pre-Deploy States:")
	for group, depthToResourceName := range groupToDepthToResourceName {
		for depth, resourceNames := range depthToResourceName {
			for _, resourceName := range resourceNames {
				fmt.Printf(
					"Group %d -> Depth %d -> %s -> %s\n",
					group,
					depth,
					resourceName,
					resourceNameToState[resourceName],
				)
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
	resourceNameToDeployState *resources.NameToDeployStateContainer,
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
				if resourceNameToDeployState.M[resourceNameAtDepthToCheck] == resources.DeployState(resources.PENDING) && !helpers.IsInSlice(result, dependencyName) {
					result = append(result, resourceNameAtDepthToCheck)
				}
			}
		}
		depthToCheck--
	}

	return result
}

type numOfResourcesInGroupToDeploy int

func setNumOfResourcesInGroupToDeploy(
	groupToResourceNames resources.GroupToNames,
	resourceNameToState resources.NameToState,
	group int,
) numOfResourcesInGroupToDeploy {
	result := numOfResourcesInGroupToDeploy(0)
	for _, resourceName := range groupToResourceNames[group] {
		if resourceNameToState[resourceName] != resources.State(resources.UNCHANGED) {
			result++
		}
	}
	return result
}
