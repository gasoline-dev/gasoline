package cmd

import (
	"fmt"
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

		resourceContainerSubDirPaths, err := resources.GetContainerSubDirPaths(viper.GetString("resourceContainerDir"))
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
			return
		}

		err = resources.ValidateContainerSubDirContents(resourceContainerSubDirPaths)
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
			return
		}

		resourceIndexBuildFilePaths, err := resources.GetIndexBuildFilePaths(resourceContainerSubDirPaths)
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
			return
		}

		resourceIndexBuildFileConfigs, err := resources.GetIndexBuildFileConfigs(resourceIndexBuildFilePaths)
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
			return
		}

		fmt.Println(resourceIndexBuildFileConfigs)

		resourcePackageJsons, err := resources.GetPackageJsons(resourceContainerSubDirPaths)
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
			return
		}

		fmt.Println(resourcePackageJsons)

		resourcePackageJsonsNameSet := resources.SetPackageJsonsNameSet(resourcePackageJsons)

		fmt.Println(resourcePackageJsonsNameSet)

		resourcePackageJsonsNameToResourceIdMap := resources.SetPackageJsonNameToResourceIdMap(resourcePackageJsons, resourceIndexBuildFileConfigs)

		fmt.Println(resourcePackageJsonsNameToResourceIdMap)

		resourceDependencyIDs := resources.SetDependencyIDs(resourcePackageJsons, resourcePackageJsonsNameToResourceIdMap, resourcePackageJsonsNameSet)

		fmt.Println(resourceDependencyIDs)

		resourceMap := resources.SetMap(resourceIndexBuildFileConfigs, resourceDependencyIDs)

		fmt.Println(resourceMap)
	},
}
