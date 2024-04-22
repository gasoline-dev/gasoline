package resources

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"gas/internal/helpers"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
)

//go:embed embed/get-index-build-file-configs.js
var getIndexBuildFileConfigsEmbed embed.FS

type ResourceContainerSubdirPaths []string

func GetContainerSubdirPaths(resourceContainerDir string) (ResourceContainerSubdirPaths, error) {
	var result ResourceContainerSubdirPaths

	entries, err := os.ReadDir(resourceContainerDir)
	if err != nil {
		return nil, fmt.Errorf("unable to read resource container dir %s", resourceContainerDir)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			result = append(result, filepath.Join(resourceContainerDir, entry.Name()))
		}
	}

	return result, nil
}

type ResourceIndexFilePaths = []string

func GetIndexFilePaths(resourceContainerSubdirPaths ResourceContainerSubdirPaths) (ResourceIndexFilePaths, error) {
	var result ResourceIndexFilePaths

	pattern := regexp.MustCompile(`^_[^.]+\.[^.]+\.[^.]+\.index\.ts$`)

	for _, subdirPath := range resourceContainerSubdirPaths {
		srcPath := filepath.Join(subdirPath, "src")

		files, err := os.ReadDir(srcPath)
		if err != nil {
			return nil, err
		}

		for _, file := range files {
			fmt.Println((file.Name()))
			if !file.IsDir() && pattern.MatchString(file.Name()) {
				result = append(result, filepath.Join(srcPath, file.Name()))
			}
		}
	}

	return result, nil
}

type ResourceIndexBuildFileConfigs = []Config

type Config struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	KV   []struct {
		Binding string `json:"binding"`
	} `json:"kv,omitempty"`
}

func GetIndexBuildFileConfigs(resourceIndexBuildFilePaths ResourceIndexBuildFilePaths) (ResourceIndexBuildFileConfigs, error) {
	var result ResourceIndexBuildFileConfigs

	embedPath := "embed/get-index-build-file-configs.js"

	content, err := getIndexBuildFileConfigsEmbed.ReadFile(embedPath)
	if err != nil {
		return result, fmt.Errorf("unable to read embed %s", embedPath)
	}

	nodeCmd := exec.Command("node", "--input-type=module")
	filePaths := strings.Join(resourceIndexBuildFilePaths, ",")
	nodeCmd.Env = append(nodeCmd.Env, "FILE_PATHS="+filePaths)
	nodeCmd.Stdin = bytes.NewReader(content)
	output, err := nodeCmd.CombinedOutput()
	if err != nil {
		return result, fmt.Errorf("unable to execute embed %s\n%s", embedPath, output)
	}

	strOutput := strings.TrimSpace((string(output)))

	jsError := "Error: unable to get exports\n"

	if strings.Contains(strOutput, jsError) {
		strOutput = strings.Replace(strOutput, jsError, "", 1)

		return result, fmt.Errorf("unable to get exports in file %s\n%s", "some file path", strOutput)
	}

	err = json.Unmarshal([]byte(strOutput), &result)
	if err != nil {
		return result, fmt.Errorf("unable to parse JSON\n%v", err)
	}

	return result, nil
}

type ResourceIndexBuildFilePaths = []string

func GetIndexBuildFilePaths(resourceContainerSubdirPaths ResourceContainerSubdirPaths) (ResourceIndexBuildFilePaths, error) {
	var result ResourceIndexBuildFilePaths

	pattern := regexp.MustCompile(`^_[^.]+\.[^.]+\.[^.]+\.index\.js$`)

	for _, subdirPath := range resourceContainerSubdirPaths {
		buildPath := filepath.Join(subdirPath, "build")

		files, err := os.ReadDir(buildPath)
		if err != nil {
			return nil, err
		}

		for _, file := range files {
			if !file.IsDir() && pattern.MatchString(file.Name()) {
				result = append(result, filepath.Join(buildPath, file.Name()))
			}
		}
	}

	return result, nil
}

type ResourcePackageJsons []PackageJson

type PackageJson struct {
	Name            string            `json:"name"`
	Main            string            `json:"main"`
	Types           string            `json:"types"`
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies,omitempty"`
	DevDependencies map[string]string `json:"devDependencies,omitempty"`
}

func GetPackageJsons(resourceContainerSubdirPaths ResourceContainerSubdirPaths) (ResourcePackageJsons, error) {
	var result ResourcePackageJsons

	for _, subdirPath := range resourceContainerSubdirPaths {
		packageJsonPath := filepath.Join(subdirPath, "package.json")

		data, err := os.ReadFile(packageJsonPath)
		if err != nil {
			return nil, fmt.Errorf("unable to read file %s\n%v", packageJsonPath, err)
		}

		var packageJson PackageJson
		err = json.Unmarshal(data, &packageJson)
		if err != nil {
			return nil, fmt.Errorf("unable to parse %s\n%v", packageJsonPath, err)
		}

		result = append(result, packageJson)
	}

	return result, nil
}

type ResourcesUpJson ResourceIDToData

func GetUpJson(resourcesUpJsonPath string) (ResourcesUpJson, error) {
	var result ResourcesUpJson
	err := helpers.UnmarshallFile(resourcesUpJsonPath, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

type ResourceDependencyIDs [][]string

func SetDependencyIDs(packageJsons ResourcePackageJsons, packageJsonNameToResourceIDMap PackageJsonNameToResourceID, packageJsonsNameSet PackageJsonNameToBool) ResourceDependencyIDs {
	var result ResourceDependencyIDs
	for _, packageJson := range packageJsons {
		var internalDependencies []string
		for dependency := range packageJson.Dependencies {
			resourceID, ok := packageJsonNameToResourceIDMap[dependency]
			if ok && packageJsonsNameSet[dependency] {
				internalDependencies = append(internalDependencies, resourceID)
			}
		}
		result = append(result, internalDependencies)
	}
	return result
}

type ResourceIDToInDegrees map[string]int

func SetIDToInDegrees(resourceMap ResourceIDToData) ResourceIDToInDegrees {
	result := make(ResourceIDToInDegrees)
	for _, resource := range resourceMap {
		for _, dep := range resource.Dependencies {
			result[dep]++
		}
	}
	for resourceID := range resourceMap {
		if _, exists := result[resourceID]; !exists {
			result[resourceID] = 0
		}
	}
	return result
}

type ResourceIDToData map[string]Resource

type Resource struct {
	Type         string
	Config       Config
	Dependencies []string
}

func SetIDToData(indexBuildFileConfigs ResourceIndexBuildFileConfigs, dependencyIDs ResourceDependencyIDs) ResourceIDToData {
	result := make(ResourceIDToData)
	for index, config := range indexBuildFileConfigs {
		result[config.ID] = Resource{
			Type:         strings.Split(config.ID, ":")[2],
			Config:       config,
			Dependencies: dependencyIDs[index],
		}
	}
	return result
}

type PackageJsonNameToResourceID map[string]string

func SetPackageJsonNameToID(packageJsons ResourcePackageJsons, indexBuildFileConfigs ResourceIndexBuildFileConfigs) PackageJsonNameToResourceID {
	result := make(PackageJsonNameToResourceID)
	for index, packageJson := range packageJsons {
		result[packageJson.Name] = indexBuildFileConfigs[index].ID
	}
	return result
}

type PackageJsonNameToBool map[string]bool

func SetPackageJsonNameToBool(packageJsons ResourcePackageJsons) PackageJsonNameToBool {
	result := make(PackageJsonNameToBool)
	for _, packageJson := range packageJsons {
		result[packageJson.Name] = true
	}
	return result
}

type State string

const (
	Created State = "CREATED"
	Deleted State = "DELETED"
	Updated State = "UPDATED"
)

type ResourceIDToState map[string]State

func SetIDStateMap(upJson ResourcesUpJson, currResourceMap ResourceIDToData) ResourceIDToState {
	result := make(ResourceIDToState)

	for upJsonResourceID := range upJson {
		if _, exists := currResourceMap[upJsonResourceID]; !exists {
			result[upJsonResourceID] = "DELETED"
		}
	}

	for currResourceID, currResource := range currResourceMap {
		if _, exists := upJson[currResourceID]; !exists {
			result[currResourceID] = "CREATED"
		} else {
			upResource := upJson[currResourceID]
			if !IsResourceEqual(upResource, currResource) {
				result[currResourceID] = "UPDATED"
			}
		}
	}

	return result
}

func IsResourceEqual(resource1, resource2 Resource) bool {
	if resource1.Type != resource2.Type {
		return false
	}
	if !reflect.DeepEqual(resource1.Config, resource2.Config) {
		return false
	}
	if !reflect.DeepEqual(resource1.Dependencies, resource2.Dependencies) {
		return false
	}
	return true
}

/*
type ResourceGraph struct {
	AdjacenciesMap map[string][]string
	InDegreesMap   map[string]int
	LevelsMap      map[int][]string
}
*/

//func NewGraph(resourceMap ResourceMap) *ResourceGraph {
// result := &ResourceGraph{
// 	AdjacenciesMap: make(map[string][]string),
// 	InDegreesMap:   make(map[string]int),
// 	LevelsMap:      make(map[int][]string),
// }

// for resourceID, resource := range resourceMap {
/*
	if len(resource.Dependencies) > 0 {
		for _, dependencyResourceID := range resource.Dependencies {
			result.AddEdge(resourceID, dependencyResourceID)
			// replace with actual
		}
	} else {
		result.InDegreesMap[resourceID] = 0
	}
*/
// 	for _, dependencyResourceID := range resource.Dependencies {
// 		result.AddEdge(resourceID, dependencyResourceID)
// 	}
// }

// Set in degrees to 0 for resources that are only ever
// source nodes and never neighbor resources.
/*
	for resource := range result.AdjacenciesMap {
		if _, exists := result.InDegreesMap[resource]; !exists {
			result.InDegreesMap[resource] = 0
		}
	}
*/

//return result
//}

// func (resourceGraph *ResourceGraph) AddEdge(sourceNode, neighborNode string) {
// 	resourceGraph.AdjacenciesMap[sourceNode] = append(resourceGraph.AdjacenciesMap[sourceNode], neighborNode)
// 	resourceGraph.InDegreesMap[neighborNode]++
// }

// func (resourceGraph *ResourceGraph) CalculateLevels() error {
// 	queue := make([]string, 0)
// 	processedCount := 0

// 	// Map to hold temporary levels with reversed order
// 	tempLevels := make(map[int][]string)

// 	// Start with nodes that have no incoming edges
// 	for node, inDegree := range resourceGraph.InDegreesMap {
// 		if inDegree == 0 {
// 			queue = append(queue, node)
// 			tempLevels[0] = append(tempLevels[0], node) // Initially no dependencies
// 		}
// 	}

// 	level := 0
// 	for len(queue) > 0 {
// 		nextLevelNodes := make([]string, 0)
// 		for _, node := range queue {
// 			processedCount++
// 			for _, neighborNode := range resourceGraph.AdjacenciesMap[node] {
// 				resourceGraph.InDegreesMap[neighborNode]--
// 				if resourceGraph.InDegreesMap[neighborNode] == 0 {
// 					nextLevelNodes = append(nextLevelNodes, neighborNode)
// 				}
// 			}
// 		}
// 		if len(nextLevelNodes) > 0 {
// 			level++
// 			tempLevels[level] = nextLevelNodes
// 			queue = nextLevelNodes
// 		} else {
// 			queue = nil
// 		}
// 	}

// 	// Reverse the keys for tempLevels to correct order
// 	maxLevel := level // Get the highest level assigned
// 	for l := 0; l <= maxLevel; l++ {
// 		resourceGraph.LevelsMap[maxLevel-l] = tempLevels[l]
// 	}

// 	if processedCount != len(resourceGraph.InDegreesMap) {
// 		return fmt.Errorf("unable to calculate levels because the graph contains a cycle")
// 	}
// 	return nil
// }

func ValidateContainerSubdirContents(subdirPaths []string) error {
	indexTsNamePattern := regexp.MustCompile(`^_[^.]+\.[^.]+\.[^.]+\.index\.ts$`)
	indexJsNamePattern := regexp.MustCompile(`^_[^.]+\.[^.]+\.[^.]+\.index\.js$`)

	for _, subdirPath := range subdirPaths {
		if _, err := os.Stat(filepath.Join(subdirPath, "package.json")); os.IsNotExist(err) {
			return fmt.Errorf("unable to find package.json in %s", subdirPath)
		}

		var indexTsParentDirPath = filepath.Join(subdirPath, "src")
		var indexJsParentDirPath = filepath.Join(subdirPath, "build")

		foundIndexTs := false
		foundIndexJs := false

		err := filepath.Walk(subdirPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !foundIndexTs && indexTsNamePattern.MatchString(info.Name()) && filepath.Dir(path) == indexTsParentDirPath {
				foundIndexTs = true
			}

			if !foundIndexJs && indexJsNamePattern.MatchString(info.Name()) && filepath.Dir(path) == indexJsParentDirPath {
				foundIndexJs = true
			}

			return nil
		})

		if err != nil {
			return fmt.Errorf("unable to walk path %q: %v", subdirPath, err)
		}

		if !foundIndexTs {
			return fmt.Errorf("unable to find resource index.ts file in %s", indexTsParentDirPath)
		}

		if !foundIndexJs {
			return fmt.Errorf("unable to find resource index.js file in %s", indexJsParentDirPath)
		}
	}

	return nil
}
