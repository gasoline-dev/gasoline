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

		resourceIDToDepth := resources.SetIDToDepth(depthToResourceID)
		fmt.Println("resource ID to depth")
		helpers.PrettyPrint(resourceIDToDepth)

		groupToDepthToResourceIDs := resources.SetGroupToDepthToResourceIDs(resourceIDToGroup, resourceIDToDepth)
		fmt.Println("group to depth to resource IDs")
		helpers.PrettyPrint(groupToDepthToResourceIDs)

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

		resourceIDToState := resources.SetIDToStateMap(resourcesUpJson, currResourceIDToData)
		fmt.Println("resource id to state")
		helpers.PrettyPrint(resourceIDToState)

		stateToResourceIDs := resources.SetStateToResourceIDs(resourceIDToState)
		fmt.Println("resource to resource IDs")
		helpers.PrettyPrint(stateToResourceIDs)

		hasResourceIDsToDeploy := resources.HasIDsToDeploy(stateToResourceIDs)
		if !hasResourceIDsToDeploy {
			fmt.Println("No resource changes to deploy")
			return
		}

		groupsWithStateChanges := resources.SetGroupsWithStateChanges(resourceIDToGroup, resourceIDToState)
		fmt.Println("groups with state changes")
		helpers.PrettyPrint(groupsWithStateChanges)

		groupsToResourceIDs := resources.SetGroupToResourceIDs(resourceIDToGroup)
		fmt.Println("groups to resource IDs")
		helpers.PrettyPrint(groupsToResourceIDs)

		groupToHighestDeployDepth := resources.SetGroupToHighestDeployDepth(
			resourceIDToDepth,
			resourceIDToState,
			groupsWithStateChanges,
			groupsToResourceIDs,
		)
		fmt.Println("group to deploy depth")
		helpers.PrettyPrint(groupToHighestDeployDepth)

		resourceIDToDeployState := resources.SetResourceIDToDeployStateOfPending(resourceIDToState)
		fmt.Println("initial deploy state")
		helpers.PrettyPrint(resourceIDToDeployState)

		resources.LogIDPreDeploymentStates(groupToDepthToResourceIDs, resourceIDToState)

		numOfGroupsToDeploy := len(groupsWithStateChanges)

		numOfGroupsFinishedDeploying := 0
		//numOfGroupsFinishedDeployingWithError := 0

		type ProcessResourceChan chan bool

		processResourceChan := make(ProcessResourceChan)

		processResource := func(processResourceChan ProcessResourceChan, resourceID string) {
			fmt.Printf("Processing resource ID %s\n", resourceID)
			time.Sleep(time.Second)
			processResourceChan <- true
		}

		type ProcessGroupChan chan any

		processGroupChan := make(ProcessGroupChan)

		processGroup := func(processGroupChan ProcessGroupChan, group int) {
			highestGroupDeployDepth := groupToHighestDeployDepth[group]

			initialGroupResourceIDsToDeploy := groupToDepthToResourceIDs[group][highestGroupDeployDepth]

			for _, resourceID := range initialGroupResourceIDsToDeploy {
				go processResource(processResourceChan, resourceID)
			}

			for msg := range processResourceChan {
				//processGroupChan <- "hello"
				if msg {
					fmt.Println("Deployed resource")
					processGroupChan <- "hello"
					break
				}
			}

			/*
				fmt.Printf("processing group: %v\n", group)
					time.Sleep(time.Second)
					processGroupChan <- "hello"
			*/
		}

		for _, group := range groupsWithStateChanges {
			go processGroup(processGroupChan, group)
		}

		for msg := range processGroupChan {
			numOfGroupsFinishedDeploying++
			fmt.Println(msg)
			if numOfGroupsToDeploy == numOfGroupsFinishedDeploying {
				fmt.Println("FINISHED DEPLOYING")
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
