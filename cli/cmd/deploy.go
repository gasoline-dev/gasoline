package cmd

import (
	"encoding/json"
	"fmt"
	"gas/internal/helpers"
	resources "gas/internal/resources"
	"log"
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

		//resourceStateMap := resources.SetStateMap(make(resources.ResourceMap), currResourceMap)
		//fmt.Println(resourceStateMap)

		// TODO: All the this stuff can be in resources.go as Up.

		// TODO: json path can be configged?
		resourcesUpJsonPath := "gas.up.json"

		isResourcesUpJsonPresent := helpers.IsFilePresent(resourcesUpJsonPath)

		if !isResourcesUpJsonPresent {
			file, err := os.Create(resourcesUpJsonPath)
			if err != nil {
				fmt.Printf("Error: unable to open %s\n%v", resourcesUpJsonPath, err)
				os.Exit(1)
				return
			}

			_, err = file.WriteString("{}")
			if err != nil {
				fmt.Printf("Error: unable to write %s\n%v", resourcesUpJsonPath, err)
			}

			if err = file.Close(); err != nil {
				log.Fatalf("Error: failed to close %s\n%v", resourcesUpJsonPath, err)
			}
		}

		data, err := os.ReadFile(resourcesUpJsonPath)
		if err != nil {
			fmt.Printf("Error: unable to read %s\n%v", resourcesUpJsonPath, err)
			os.Exit(1)
			return
		}

		var ResourcesUpMap ResourcesUpMap
		err = json.Unmarshal(data, &ResourcesUpMap)
		if err != nil {
			fmt.Printf("Error: unable to parse %s\n%v", resourcesUpJsonPath, err)
			os.Exit(1)
			return
		}

		helpers.PrettyPrint(ResourcesUpMap)

		// TODO: Merge this into SetStateMap
		// use the ResourceUpMap type
		// the isresourceEqual func will have to
		// ignore State on the ResourceUpMap

		testStateMap := make(resources.StateMap)

		for resourceID := range ResourcesUpMap {
			if _, ok := currResourceMap[resourceID]; !ok {
				if ResourcesUpMap[resourceID].State == "CREATED" || ResourcesUpMap[resourceID].State == "UPDATED" {
					testStateMap[resourceID] = "DELETED"
				}
			}
		}

		for currResourceID := range currResourceMap {
			if _, ok := ResourcesUpMap[currResourceID]; !ok {
				testStateMap[currResourceID] = "CREATED"
			} else {
				//
			}
		}

		//for currResourceID := range currResourceMap {
		//if _, exists := resourcesJson[currResourceID]; !exists {
		//testStateMap[currResourceID] = "CREATED"
		///} else {
		// prevResource := prevResourceMap[currResourceID]
		/*
			if !isResourceEqual(prevResource, currResource) {
				testStateMap[currResourceID] = "UPDATED"
			}
		*/
		//}
		//}

		helpers.PrettyPrint(testStateMap)

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
