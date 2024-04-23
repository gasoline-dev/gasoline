package cmd

import (
	"fmt"
	"gas/internal/helpers"
	resources "gas/internal/resources"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

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
		fmt.Println(currResourceIndexBuildFilePaths)

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

		currResourcePackageJsonNameToBool := resources.SetPackageJsonNameToTrue(currResourcePackageJsons)

		currResourcePackageJsonNameToID := resources.SetPackageJsonNameToID(currResourcePackageJsons, currResourceIndexBuildFileConfigs)

		currResourceDependencyIDs := resources.SetDependencyIDs(currResourcePackageJsons, currResourcePackageJsonNameToID, currResourcePackageJsonNameToBool)

		currResourceIDToData := resources.SetIDToData(currResourceIndexBuildFileConfigs, currResourceDependencyIDs)

		helpers.PrettyPrint(currResourceIDToData)

		resourceIDToInDegrees := resources.SetIDToInDegrees(currResourceIDToData)
		helpers.PrettyPrint(resourceIDToInDegrees)

		resourceIDsWithInDegreesOfZero := resources.SetIDsWithInDegreesOf(resourceIDToInDegrees, 0)

		fmt.Println(resourceIDsWithInDegreesOfZero)

		// TODO: Might not need this
		resourceIDs := resources.SetIDs(currResourceIDToData)
		fmt.Println(resourceIDs)

		resourceIDToIntermediateIDs := resources.SetIDToIntermediateIDs(currResourceIDToData)
		fmt.Println("resource ID to intermediates")
		helpers.PrettyPrint(resourceIDToIntermediateIDs)

		resourceIDToGroup := resources.SetIDToGroup(resourceIDsWithInDegreesOfZero, resourceIDToIntermediateIDs)
		fmt.Println("resource ID to group")
		helpers.PrettyPrint(resourceIDToGroup)

		depthToResourceID := resources.SetDepthToResourceID(resourceIDs, currResourceIDToData, resourceIDsWithInDegreesOfZero)
		fmt.Println("depth to resource ID")
		helpers.PrettyPrint(depthToResourceID)

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
		helpers.PrettyPrint(resourcesUpJson)

		resourceIDToDeployState := setResourceIDToDeployState(resourcesUpJson, currResourceIDToData)
		helpers.PrettyPrint(resourceIDToDeployState)

		type ProcessResourceChan chan any

		processResourceChan := make(ProcessResourceChan)

		processResource := func(processResourceChan ProcessResourceChan, resourceID string) {
			fmt.Printf("processing resource: %s\n", resourceID)
			time.Sleep(time.Second)
			processResourceChan <- true
		}

		for resourceID, deployState := range resourceIDToDeployState {
			if resourceIDToInDegrees[resourceID] == 0 && deployState == "CREATE_PENDING" {
				go processResource(processResourceChan, resourceID)
			}
		}

		resourceCount := 0
		for range processResourceChan {
			fmt.Println("Resource processed")
			resourceCount++
			if resourceCount == 2 {
				break
			}
		}

		/*
			api, err := cloudflare.NewWithAPIToken(os.Getenv("CLOUDFLARE_API_TOKEN"))
			if err != nil {
				log.Fatal(err)
			}

			// Most API calls require a Context
			ctx := context.Background()

			// Fetch user details on the account
			u, err := api.UserDetails(ctx)
			if err != nil {
				log.Fatal(err)
			}
			// Print user details
			fmt.Println(u)
		*/
		/*
			resourceToDeployStateMap := setDeployStateMap(resourcesUpJson, currResourceMap)

			resourceIDToGraphLevelMap := setResourceIDToGraphLevelMap(resourceGraph)

			logResourcePreDeployStates(resourceGraph, resourceToDeployStateMap)

			resourceToDoneChannelMap := make(map[string]chan bool)
			for _, resources := range resourceGraph.LevelsMap {
				for _, resource := range resources {
					resourceToDoneChannelMap[resource] = make(chan bool)
				}
			}

			for _, resourceID := range resourceGraph.LevelsMap[0] {
				transitionResourceToDeployStateMapOnStart(resourceID, resourceToDeployStateMap)
				logResourceDeployState(resourceID, resourceToDeployStateMap, resourceIDToGraphLevelMap)
				go processResource(resourceID, resourceToDoneChannelMap[resourceID])
			}

			for level := 0; level < len(resourceGraph.LevelsMap); level++ {
				for _, resource := range resourceGraph.LevelsMap[level] {
					<-resourceToDoneChannelMap[resource]
					fmt.Printf("Resource %s completed\n", resource)
					if level+1 < len(resourceGraph.LevelsMap) {
						for _, nextResourceID := range resourceGraph.LevelsMap[level+1] {
							transitionResourceToDeployStateMapOnStart(nextResourceID, resourceToDeployStateMap)
							logResourceDeployState(nextResourceID, resourceToDeployStateMap, resourceIDToGraphLevelMap)
							go processResource(nextResourceID, resourceToDoneChannelMap[nextResourceID])
						}
					}
				}
			}
		*/
	},
}

func processResource(resourceID string, doneChan chan bool) {
	fmt.Printf("Processing resource ID %s\n", resourceID)
	time.Sleep(time.Second)
	doneChan <- true
}

type DeployState string

const (
	CreatePending    DeployState = "CREATE_PENDING"
	DeletePending    DeployState = "DELETE_PENDING"
	UpdatePending    DeployState = "UPDATE_PENDING"
	CreateInProgress DeployState = "CREATE_IN_PROGRESS"
	DeleteInProgress DeployState = "DELETE_IN_PROGRESS"
	UpdateInProgress DeployState = "UPDATE_IN_PROGRESS"
	CreateFailed     DeployState = "CREATE_FAILED"
	DeleteFailed     DeployState = "DELETE_FAILED"
	UpdateFailed     DeployState = "UPDATE_FAILED"
	CreateSuccess    DeployState = "CREATE_SUCCESS"
	DeleteSuccess    DeployState = "DELETE_SUCCESS"
	UpdateSuccess    DeployState = "UPDATE_SUCCESS"
)

type ResourceIDToDeployState = map[string]DeployState

func setResourceIDToDeployState(upJson resources.ResourcesUpJson, resourceIDToData resources.ResourceIDToData) ResourceIDToDeployState {
	result := make(ResourceIDToDeployState)

	for upJsonResourceID := range upJson {
		if _, exists := resourceIDToData[upJsonResourceID]; !exists {
			result[upJsonResourceID] = DeletePending
		}
	}

	for currResourceID, currResource := range resourceIDToData {
		if _, exists := upJson[currResourceID]; !exists {
			result[currResourceID] = CreatePending
		} else {
			upResource := upJson[currResourceID]
			if !resources.IsResourceEqual(upResource, currResource) {
				result[currResourceID] = UpdatePending
			}
		}
	}

	return result
}

// type ResourceIDToGraphLevelMap = map[string]int

// func setResourceIDToGraphLevelMap(resourceGraph *resources.ResourceGraph) ResourceIDToGraphLevelMap {
// 	result := make(ResourceIDToGraphLevelMap)

// 	for level := 0; level < len(resourceGraph.LevelsMap); level++ {
// 		for _, resourceID := range resourceGraph.LevelsMap[level] {
// 			result[resourceID] = level
// 		}
// 	}

// 	return result
// }

// func logResourcePreDeployStates(resourceGraph *resources.ResourceGraph, resourceToDeployStateMap ResourceToDeployStateMap) {
// 	for level := 0; level < len(resourceGraph.LevelsMap); level++ {
// 		for _, resource := range resourceGraph.LevelsMap[level] {
// 			fmt.Printf("Level %d -> %s -> %s\n", level, resource, resourceToDeployStateMap[resource])
// 		}
// 	}
// }

func transitionResourceToDeployStateMapOnStart(resourceID string, resourceToDeployStateMap ResourceIDToDeployState) {
	switch state := resourceToDeployStateMap[resourceID]; state {
	case CreatePending:
		resourceToDeployStateMap[resourceID] = CreateInProgress
	case DeletePending:
		resourceToDeployStateMap[resourceID] = DeleteInProgress
	case UpdatePending:
		resourceToDeployStateMap[resourceID] = UpdateInProgress
	}
}

// func logResourceDeployState(resourceID string, resourceToDeployStateMap ResourceToDeployStateMap, resourceIDToGraphLevelMap ResourceIDToGraphLevelMap) {
// 	fmt.Printf("Level %d -> %s -> %s\n", resourceIDToGraphLevelMap[resourceID], resourceID, resourceToDeployStateMap[resourceID])
// }
