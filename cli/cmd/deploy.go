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
		fmt.Println(viper.GetString("CLOUDFLARE_ACCOUNT_ID"))

		currResourceContainerSubDirPaths, err := resources.GetContainerSubDirPaths(viper.GetString("resourceContainerDir"))
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
			return
		}

		err = resources.ValidateContainerSubDirContents(currResourceContainerSubDirPaths)
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
			return
		}

		currResourceIndexBuildFilePaths, err := resources.GetIndexBuildFilePaths(currResourceContainerSubDirPaths)
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

		currResourcePackageJsons, err := resources.GetPackageJsons(currResourceContainerSubDirPaths)
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
			return
		}

		currResourcePackageJsonsNameSet := resources.SetPackageJsonsNameSet(currResourcePackageJsons)

		currResourcePackageJsonsNameToIDMap := resources.SetPackageJsonNameToIDMap(currResourcePackageJsons, currResourceIndexBuildFileConfigs)

		currResourceDependencyIDs := resources.SetDependencyIDs(currResourcePackageJsons, currResourcePackageJsonsNameToIDMap, currResourcePackageJsonsNameSet)

		currResourceMap := resources.SetMap(currResourceIndexBuildFileConfigs, currResourceDependencyIDs)

		resourceGraph := resources.NewGraph(currResourceMap)

		err = resourceGraph.CalculateLevels()
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
			return
		}

		helpers.PrettyPrint(resourceGraph)

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

		resourceStateMap := resources.SetStateMap(resourcesUpJson, currResourceMap)
		helpers.PrettyPrint(resourceStateMap)

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

		logPreDeploymentStates(resourceGraph, resourceStateMap)

		resourceToDoneChannelMap := make(map[string]chan bool)
		for _, resources := range resourceGraph.LevelsMap {
			for _, resource := range resources {
				resourceToDoneChannelMap[resource] = make(chan bool)
			}
		}

		// Start the tasks for level 0
		for _, resourceID := range resourceGraph.LevelsMap[0] {
			go processResource(resourceID, resourceToDoneChannelMap[resourceID])
		}

		// Listen for task completions and trigger subsequent tasks
		for level := 0; level < len(resourceGraph.LevelsMap); level++ {
			resources := resourceGraph.LevelsMap[level]
			for _, resource := range resources {
				<-resourceToDoneChannelMap[resource] // Wait for each task to complete
				fmt.Printf("Resource %s completed\n", resource)
				if level+1 < len(resourceGraph.LevelsMap) {
					for _, nextTask := range resourceGraph.LevelsMap[level+1] {
						go processResource(nextTask, resourceToDoneChannelMap[nextTask])
					}
				}
			}
		}
	},
}

func processResource(resourceID string, doneChan chan bool) {
	fmt.Printf("Processing resource ID %s\n", resourceID)
	time.Sleep(time.Second)
	doneChan <- true
}

func logPreDeploymentStates(resourceGraph *resources.ResourceGraph, resourceStateMap resources.ResourcesStateMap) {
	fmt.Println("# Pre-deployment states:")
	for level := 0; level < len(resourceGraph.LevelsMap); level++ {
		for _, resource := range resourceGraph.LevelsMap[level] {
			fmt.Printf("Level %d -> %s -> %s\n", level, resource, resourceStateMap[resource])
		}
	}
}
