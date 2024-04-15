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

		helpers.PrettyPrint(currResourceMap)

		// TODO: All of the above needs to be done for prevResource as well.
		// Then merge prevResourceMap with currResourceMap using the
		// mergeMaps helper. They can be merged as resourcesMap.
		// The reason they'll be merged is because prev may have deleted
		// keys that don't exist in curr. Those deleted keys have to be
		// accounted for.

		resourceGraph := resources.NewGraph(currResourceMap)

		err = resourceGraph.CalculateLevels()
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
			return
		}

		helpers.PrettyPrint(resourceGraph)

		resourceStateMap := resources.SetStateMap(make(resources.ResourceMap), currResourceMap)
		fmt.Println(resourceStateMap)

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
	},
}

func processTask(resourceID string, doneChan chan bool) {
	fmt.Printf("Processing resource ID %s\n", resourceID)
	time.Sleep(time.Second)
	doneChan <- true
}
