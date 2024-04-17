package cmd

import (
	"fmt"
	"gas/internal/helpers"
	resources "gas/internal/resources"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type ResourcesUpMap map[string]struct {
	resources.Resource
	State string
}

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy resources to the cloud",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Deploying resources to the cloud")

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
				// Create a map to keep track of which tasks are complete
			completionChannels := make(map[string]chan bool)
			for _, tasks := range resourceGraph.LevelsMap {
				for _, task := range tasks {
					completionChannels[task] = make(chan bool)
				}
			}

				// Start the tasks for level 0
				for _, resourceID := range resourceGraph.LevelsMap[0] {
					go processTask(resourceID, completionChannels[resourceID])
				}

				// Listen for task completions and trigger subsequent tasks
				for level := 0; level < len(resourceGraph.LevelsMap); level++ {
					tasks := resourceGraph.LevelsMap[level]
					for _, task := range tasks {
						<-completionChannels[task] // Wait for each task to complete
						fmt.Printf("Task %s completed\n", task)
						if level+1 < len(resourceGraph.LevelsMap) {
							for _, nextTask := range resourceGraph.LevelsMap[level+1] {
								go processTask(nextTask, completionChannels[nextTask])
							}
						}
					}
				}
		*/
	},
}

/*
func processTask(resourceID string, doneChan chan bool) {
	fmt.Printf("Processing resource ID %s\n", resourceID)
	time.Sleep(time.Second)
	doneChan <- true
}
*/
