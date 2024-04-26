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

		resourceIDToDeployState := resources.SetIDToDeployStateOfPending(resourceIDToState)
		fmt.Println("initial deploy state")
		helpers.PrettyPrint(resourceIDToDeployState)

		numOfGroupsToDeploy := len(groupsWithStateChanges)

		numOfGroupsFinishedDeploying := 0

		type DeployResourceOkChan chan bool

		deployResourceOkChan := make(DeployResourceOkChan)

		deployResource := func(deployResourceOkChan DeployResourceOkChan, group int, depth int, resourceID string) {
			resources.SetIDToDeployStateOnStart(resourceIDToDeployState, resourceIDToState, resourceID)

			timestamp := time.Now().UnixMilli()

			resources.LogIDDeployState(group, depth, resourceID, timestamp, resourceIDToDeployState)

			time.Sleep(time.Second)

			fmt.Printf("Processed resource ID %s\n", resourceID)

			deployResourceOkChan <- true
		}

		type DeployGroupOkChan chan bool

		deployGroupOkChan := make(DeployGroupOkChan)

		deployGroup := func(deployGroupOkChan DeployGroupOkChan, group int) {
			highestGroupDeployDepth := groupToHighestDeployDepth[group]

			initialGroupResourceIDsToDeploy := groupToDepthToResourceIDs[group][highestGroupDeployDepth]

			/*
				         a - b - c - d
				a - c - e - f - g

				d and g deploy

				g would block d because g has higher depth.

				so on initial deploy that needs to be accounted for. it cant just start at the initial depth. it needs to start there, but also loop down.

				* change init deploy depth to highest

				start with highest deploy depth -> 6.

				groupToDepthToResourceIDs
				- add every resource at highest depth to res
				- depth to check = init depth --
				- for d t c > 0
				- for _, rid:= r at d t c
				- if rid at dtc is not dep on any rid in res and state needs to be deployed, resourceIDToState, append to res
				- end r at dtc loop
				- depth--

				SetInitialIDsToDeploy
			*/

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

					/*
						 transitionNodeDeployStateOnOk(node);

														nodeToTimeEndedAtMap[node] = Date.now();

														logNodeDeployState({
															depth,
															group,
															node,
															timestamp: nodeToTimeEndedAtMap[node],
														});
					*/
				} else {
					numOfResourcesDeployedErr++
					// if canceled num == 0 then cancel pending r's.
					// pendingResourceIDsCanceled := cancel()
					/*
						transitionNodeDeployStateOnErr(node);

																logNodeDeployState({
																	depth: nodeToDepthMap[node] as string,
																	group,
																	node,
																	timestamp: Date.now(),
																});

																cancelPendingNodes(
																	group,
																	groupToNodesMap,
																	nodeToDeployStateMap
																);

								 function cancelPendingNodes(
								group: string,
								groupToNodesMap: GroupToNodesMap,
								nodeToDeployStateMap: NodeToDeployStateMap
							): void {
								for (const node of groupToNodesMap[group]) {
									if (nodeToDeployStateMap[node] === 'PENDING') {
										nodeToDeployStateMap[node] = 'CANCELED';
									}
								}
							}
					*/
					numOfResourcesDeployedCanceled++
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
					// keep deploying
					// loop over group resources
					// if a resource has a state of PENDING
					// then check if that resource is dependent
					// on a currently deploying resource
					// or one that is pending or one that failed.
					// if not, then deploy it
					/*
						function isNodeDirectlyDependentOnActivelyDeployingNode(
								node: string,
								nodeToDirectDependenciesMap: NodeToDirectDependenciesMap,
								nodeToDeployStateMap: NodeToDeployStateMap
							): boolean {
								let result = false;
								for (const nodeToCheck of nodeToDirectDependenciesMap[node]) {
									const activeStates = [
										'CREATE_IN_PROGRESS',
										'DELETE_IN_PROGRESS',
										'PENDING',
										'UPDATE_IN_PROGRESS',
									];
									if (activeStates.includes(nodeToDeployStateMap[nodeToCheck])) {
										result = true;
										break;
									}
								}
								return result;
							}
					*/
				}
			}
		}

		for _, group := range groupsWithStateChanges {
			go deployGroup(deployGroupOkChan, group)
		}

		// numOfGroupsDeployedOk := 0
		// numOfGroupsDeployedErr := 0

		for groupDeployedOk := range deployGroupOkChan {
			// if numOfGroupsDeployedOk++
			// else numOfGroupsDeployedErr++
			// numOfGroupsFinishedDeploying := ...
			// if num finished == to deploy
			// if err > 0, os exit 1 return else
			// os exit 0.
			numOfGroupsFinishedDeploying++
			fmt.Println(groupDeployedOk)
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
	},
}
