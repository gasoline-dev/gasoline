package cmd

import (
	"fmt"
	"gas/internal/helpers"
	"gas/internal/resources"
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

		currResourcePackageJsonNameToTrue := resources.SetPackageJsonNameToTrue(currResourcePackageJsons)

		currResourcePackageJsonNameToID := resources.SetPackageJsonNameToID(currResourcePackageJsons, currResourceIndexBuildFileConfigs)

		currResourceDependencyIDs := resources.SetDependencyIDs(currResourcePackageJsons, currResourcePackageJsonNameToID, currResourcePackageJsonNameToTrue)

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

		resources.LogIDPreDeploymentStates(groupToDepthToResourceIDs, resourceIDToState)

		resourceIDToDeployState := resources.UpdateIDToDeployStateOfPending(resourceIDToState)
		fmt.Println("initial deploy state")
		helpers.PrettyPrint(resourceIDToDeployState)

		numOfGroupsToDeploy := len(groupsWithStateChanges)

		type DeployResourceOkChan chan bool

		deployResourceOkChan := make(DeployResourceOkChan)

		deployResource := func(deployResourceOkChan DeployResourceOkChan, group int, depth int, resourceID string) {
			resources.UpdateIDToDeployStateOnStart(resourceIDToDeployState, resourceIDToState, resourceID)

			timestamp := time.Now().UnixMilli()

			resources.LogIDDeployState(group, depth, resourceID, timestamp, resourceIDToDeployState)

			time.Sleep(time.Second)

			// TODO: Everything below is for state OK -> Add if-else
			// to handle on err -> UpdateResourceIDToDeployStateOnErr

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
		}

		type DeployGroupOkChan chan bool

		deployGroupOkChan := make(DeployGroupOkChan)

		deployGroup := func(deployGroupOkChan DeployGroupOkChan, group int) {
			highestGroupDeployDepth := groupToHighestDeployDepth[group]

			initialGroupResourceIDsToDeploy := resources.SetInitialGroupIDsToDeploy(highestGroupDeployDepth, group, groupToDepthToResourceIDs, currResourceIDToData, resourceIDToDeployState)

			for _, resourceID := range initialGroupResourceIDsToDeploy {
				depth := resourceIDToDepth[resourceID]
				go deployResource(deployResourceOkChan, group, depth, resourceID)
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

							// Is resource dependent on onther deploying resource?
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
								go deployResource(deployResourceOkChan, group, depth, resourceID)
							}
						}
					}
				}
			}
		}

		for _, group := range groupsWithStateChanges {
			go deployGroup(deployGroupOkChan, group)
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
					fmt.Println("Deployment failed")
					os.Exit(1)
					return

				}
				fmt.Println("Deployment succeeded")
				os.Exit(0)
				return
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
	},
}
