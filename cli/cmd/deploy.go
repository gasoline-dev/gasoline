package cmd

import (
	"fmt"
	"gas/internal/helpers"
	resources "gas/internal/resources"
	"os"

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

		fmt.Println(currResourceIndexBuildFileConfigs)

		currResourcePackageJsons, err := resources.GetPackageJsons(currResourceContainerSubDirPaths)
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
			return
		}

		fmt.Println(currResourcePackageJsons)

		currResourcePackageJsonsNameSet := resources.SetPackageJsonsNameSet(currResourcePackageJsons)

		fmt.Println(currResourcePackageJsonsNameSet)

		currResourcePackageJsonsNameToResourceIdMap := resources.SetPackageJsonNameToResourceIdMap(currResourcePackageJsons, currResourceIndexBuildFileConfigs)

		fmt.Println(currResourcePackageJsonsNameToResourceIdMap)

		currResourceDependencyIDs := resources.SetDependencyIDs(currResourcePackageJsons, currResourcePackageJsonsNameToResourceIdMap, currResourcePackageJsonsNameSet)

		fmt.Println(currResourceDependencyIDs)

		currResourceIDMap := resources.SetIDMap(currResourceIndexBuildFileConfigs, currResourceDependencyIDs)

		// TODO: All of the above needs to be done for prevResource as well.
		// Then merge prevResourceMap with currResourceMap using the
		// mergeMaps helper. They can be merged as resourcesMap.
		// The reason they'll be merged is because prev may have deleted
		// keys that don't exist in curr. Those deleted keys have to be
		// accounted for.

		fmt.Println(currResourceIDMap)

		resourceIDToUpstreamDependenciesMap := resources.SetResourceIDToUpstreamDependenciesMap(currResourceIDMap)

		helpers.PrettyPrint(resourceIDToUpstreamDependenciesMap)
	},
}
