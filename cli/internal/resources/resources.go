package resources

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
)

//go:embed embed/get-index-build-file-configs.js
var getIndexBuildFileConfigsEmbed embed.FS

type ResourceContainerSubDirPaths []string

func GetContainerSubDirPaths(resourceContainerDir string) (ResourceContainerSubDirPaths, error) {
	var result ResourceContainerSubDirPaths

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

func GetIndexFilePaths(resourceContainerSubDirPaths ResourceContainerSubDirPaths) (ResourceIndexFilePaths, error) {
	var result ResourceIndexFilePaths

	pattern := regexp.MustCompile(`^_[^.]+\.[^.]+\.[^.]+\.index\.ts$`)

	for _, subDirPath := range resourceContainerSubDirPaths {
		srcPath := filepath.Join(subDirPath, "src")

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

func GetIndexBuildFilePaths(resourceContainerSubDirPaths ResourceContainerSubDirPaths) (ResourceIndexBuildFilePaths, error) {
	var result ResourceIndexBuildFilePaths

	pattern := regexp.MustCompile(`^_[^.]+\.[^.]+\.[^.]+\.index\.js$`)

	for _, subDirPath := range resourceContainerSubDirPaths {
		buildPath := filepath.Join(subDirPath, "build")

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

func GetPackageJsons(resourceContainerSubDirPaths ResourceContainerSubDirPaths) (ResourcePackageJsons, error) {
	var result ResourcePackageJsons

	for _, subDirPath := range resourceContainerSubDirPaths {
		packageJsonPath := filepath.Join(subDirPath, "package.json")

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

type ResourceDependencyIDs [][]string

func SetDependencyIDs(packageJsons ResourcePackageJsons, packageJsonNameToResourceIDMap PackageJsonNameToResourceIDMap, packageJsonsNameSet PackageJsonsNameSet) ResourceDependencyIDs {
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

type ResourceMap map[string]Resource

type Resource struct {
	Type         string
	Config       Config
	Dependencies []string
}

func SetMap(indexBuildFileConfigs ResourceIndexBuildFileConfigs, dependencyIDs ResourceDependencyIDs) ResourceMap {
	result := make(ResourceMap)
	for index, config := range indexBuildFileConfigs {
		result[config.ID] = Resource{
			Type:         strings.Split(config.ID, ":")[2],
			Config:       config,
			Dependencies: dependencyIDs[index],
		}
	}
	return result
}

type PackageJsonNameToResourceIDMap map[string]string

func SetPackageJsonNameToIDMap(packageJsons ResourcePackageJsons, indexBuildFileConfigs ResourceIndexBuildFileConfigs) PackageJsonNameToResourceIDMap {
	result := make(PackageJsonNameToResourceIDMap)
	for index, packageJson := range packageJsons {
		result[packageJson.Name] = indexBuildFileConfigs[index].ID
	}
	return result
}

type PackageJsonsNameSet map[string]bool

func SetPackageJsonsNameSet(packageJsons ResourcePackageJsons) PackageJsonsNameSet {
	result := make(PackageJsonsNameSet)
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

type StateMap map[string]State

func SetStateMap(prevResourceMap, currResourceMap ResourceMap) StateMap {
	result := make(StateMap)

	for prevResourceID := range prevResourceMap {
		if _, exists := currResourceMap[prevResourceID]; !exists {
			result[prevResourceID] = "DELETED"
		}
	}

	for currResourceID, currResource := range currResourceMap {
		if _, exists := prevResourceMap[currResourceID]; !exists {
			result[currResourceID] = "CREATED"
		} else {
			prevResource := prevResourceMap[currResourceID]
			if !isResourceEqual(prevResource, currResource) {
				result[currResourceID] = "UPDATED"
			}
		}
	}

	return result
}

func isResourceEqual(resource1, resource2 Resource) bool {
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

type ResourceGraph struct {
	AdjacenciesMap map[string][]string
	InDegreesMap   map[string]int
	LevelsMap      map[int][]string
}

func NewGraph(resourceMap ResourceMap) *ResourceGraph {
	result := &ResourceGraph{
		AdjacenciesMap: make(map[string][]string),
		InDegreesMap:   make(map[string]int),
		LevelsMap:      make(map[int][]string),
	}

	for resourceID, resource := range resourceMap {
		for _, dependency := range resource.Dependencies {
			result.AddEdge(resourceID, dependency)
		}
	}

	// Set in degrees to 0 for nodes that are only ever
	// source nodes and never neighbor nodes.
	for node := range result.AdjacenciesMap {
		if _, exists := result.InDegreesMap[node]; !exists {
			result.InDegreesMap[node] = 0
		}
	}

	return result
}

func (resourceGraph *ResourceGraph) AddEdge(sourceNode, neighborNode string) {
	resourceGraph.AdjacenciesMap[sourceNode] = append(resourceGraph.AdjacenciesMap[sourceNode], neighborNode)
	resourceGraph.InDegreesMap[neighborNode]++
}

func (resourceGraph *ResourceGraph) CalculateLevels() error {
	queue := make([]string, 0)
	processedCount := 0

	// Map to hold temporary levels with reversed order
	tempLevels := make(map[int][]string)

	// Start with nodes that have no incoming edges
	for node, inDegree := range resourceGraph.InDegreesMap {
		if inDegree == 0 {
			queue = append(queue, node)
			tempLevels[0] = append(tempLevels[0], node) // Initially no dependencies
		}
	}

	level := 0
	for len(queue) > 0 {
		nextLevelNodes := make([]string, 0)
		for _, node := range queue {
			processedCount++
			for _, neighborNode := range resourceGraph.AdjacenciesMap[node] {
				resourceGraph.InDegreesMap[neighborNode]--
				if resourceGraph.InDegreesMap[neighborNode] == 0 {
					nextLevelNodes = append(nextLevelNodes, neighborNode)
				}
			}
		}
		if len(nextLevelNodes) > 0 {
			level++
			tempLevels[level] = nextLevelNodes
			queue = nextLevelNodes
		} else {
			queue = nil
		}
	}

	// Reverse the keys for tempLevels to correct order
	maxLevel := level // Get the highest level assigned
	for l := 0; l <= maxLevel; l++ {
		resourceGraph.LevelsMap[maxLevel-l] = tempLevels[l]
	}

	if processedCount != len(resourceGraph.InDegreesMap) {
		return fmt.Errorf("unable to calculate levels because the graph contains a cycle")
	}
	return nil
}

func ValidateContainerSubDirContents(subDirPaths []string) error {
	indexTsNamePattern := regexp.MustCompile(`^_[^.]+\.[^.]+\.[^.]+\.index\.ts$`)
	indexJsNamePattern := regexp.MustCompile(`^_[^.]+\.[^.]+\.[^.]+\.index\.js$`)

	for _, subDirPath := range subDirPaths {
		if _, err := os.Stat(filepath.Join(subDirPath, "package.json")); os.IsNotExist(err) {
			return fmt.Errorf("unable to find package.json in %s", subDirPath)
		}

		var indexTsParentDirPath = filepath.Join(subDirPath, "src")
		var indexJsParentDirPath = filepath.Join(subDirPath, "build")

		foundIndexTs := false
		foundIndexJs := false

		err := filepath.Walk(subDirPath, func(path string, info os.FileInfo, err error) error {
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
			return fmt.Errorf("unable to walk path %q: %v", subDirPath, err)
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
