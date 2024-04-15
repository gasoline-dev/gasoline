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
	"regexp"
	"strings"
)

//go:embed embed/get-index-build-file-configs.js
var getIndexBuildFileConfigsEmbed embed.FS

/*
GetContainerSubDirPaths returns a list of subdirectory paths in the
container directory. For example, ["gas/core-base-api"].
*/
func GetContainerSubDirPaths(containerDir string) ([]string, error) {
	var result []string

	entries, err := os.ReadDir(containerDir)
	if err != nil {
		return nil, fmt.Errorf("unable to read resource container dir %s", containerDir)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			result = append(result, filepath.Join(containerDir, entry.Name()))
		}
	}

	return result, nil
}

/*
GetIndexFilePaths returns a list of index file paths in the container
subdirectories. For example,
["gas/core-base-api/src/_core-base-api.v1.api.index.ts"].
*/
func GetIndexFilePaths(containerSubDirPaths []string) ([]string, error) {
	var result []string

	pattern := regexp.MustCompile(`^_[^.]+\.[^.]+\.[^.]+\.index\.ts$`)

	for _, subDirPath := range containerSubDirPaths {
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

type IndexBuildFileConfigs = []Config

type Config struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	KV   []struct {
		Binding string `json:"binding"`
	} `json:"kv,omitempty"`
}

/*
GetIndexBuildFileConfigs returns a list of index build file configs.
For example, [{"id":"core:base:cloudflare-worker:12345",
"name":"CORE_BASE_API","kv":[{"binding":"CORE_BASE_KV"}]}].
*/
func GetIndexBuildFileConfigs(indexBuildFilePaths []string) (IndexBuildFileConfigs, error) {
	var result []Config

	embedPath := "embed/get-index-build-file-configs.js"

	content, err := getIndexBuildFileConfigsEmbed.ReadFile(embedPath)
	if err != nil {
		return result, fmt.Errorf("unable to read embed %s", embedPath)
	}

	nodeCmd := exec.Command("node", "--input-type=module")
	filePaths := strings.Join(indexBuildFilePaths, ",")
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

/*
GetIndexBuildFilePaths returns a list of index build file
paths in the container subdirectories. For example,
["gas/core-base-api/build/_core-base-api.v1.api.index.js"].
*/
func GetIndexBuildFilePaths(containerSubDirPaths []string) ([]string, error) {
	var result []string

	pattern := regexp.MustCompile(`^_[^.]+\.[^.]+\.[^.]+\.index\.js$`)

	for _, subDirPath := range containerSubDirPaths {
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

type PackageJson struct {
	Name            string            `json:"name"`
	Main            string            `json:"main"`
	Types           string            `json:"types"`
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies,omitempty"`
	DevDependencies map[string]string `json:"devDependencies,omitempty"`
}

type PackageJsons []PackageJson

/*
GetPackageJsons returns a list of package.json objects
in the container subdirectories.
*/
func GetPackageJsons(resourceContainerSubDirPaths []string) (PackageJsons, error) {
	var result PackageJsons

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

type DependencyIDs [][]string

/*
SetDependencyIDs returns a list of resource dependency
IDs.

Resource dependencies are resources a resource depends on.
For example, resource core:base:cloudflare-worker:12345 might
depend on core:base:cloudflare-kv:12345.
*/
func SetDependencyIDs(packageJsons PackageJsons, packageJsonNameToResourceIdMap PackageJsonNameToResourceIdMap, packageJsonsNameSet PackageJsonsNameSet) DependencyIDs {
	var result DependencyIDs
	for _, packageJson := range packageJsons {
		var internalDependencies []string
		for dependency := range packageJson.Dependencies {
			resourceID, ok := packageJsonNameToResourceIdMap[dependency]
			if ok && packageJsonsNameSet[dependency] {
				internalDependencies = append(internalDependencies, resourceID)
			}
		}
		result = append(result, internalDependencies)
	}
	return result
}

type ResourceIDMap map[string]struct {
	Type         string
	Config       Config
	Dependencies []string
}

/*
SetIDMap returns a map of resource IDs to resource types, configs, and dependencies.
*/
func SetIDMap(indexBuildFileConfigs IndexBuildFileConfigs, dependencyIDs DependencyIDs) ResourceIDMap {
	result := make(ResourceIDMap)
	for index, config := range indexBuildFileConfigs {
		result[config.ID] = struct {
			Type         string
			Config       Config
			Dependencies []string
		}{
			Type:         strings.Split(config.ID, ":")[2],
			Config:       config,
			Dependencies: dependencyIDs[index],
		}
	}
	return result
}

type PackageJsonNameToResourceIdMap map[string]string

/*
SetPackageJsonNameToResourceIdMap returns a map of package.json names
to resource IDs.

Resource relationships are managed via each resource's package.json. For example,
package core-base-kv might be a dependency of package core-base-api. Therefore,
core-base-kv would exist in core-base-api's package.json's dependencies.

When core-base-api's package.json is processed and the core-base-kv dependency
is found, this map can look up core-base-kv's resource ID -- establishing
that resource core:base:cloudflare-kv:12345 is a dependency of
core:base:cloudflare-worker:12345.
*/
func SetPackageJsonNameToResourceIdMap(packageJsons PackageJsons, indexBuildFileConfigs IndexBuildFileConfigs) PackageJsonNameToResourceIdMap {
	result := make(map[string]string)
	for index, packageJson := range packageJsons {
		result[packageJson.Name] = indexBuildFileConfigs[index].ID
	}
	return result
}

type PackageJsonsNameSet map[string]bool

/*
SetPackageJsonsNameSet returns a set of package.json names.

This can be used to tell if a dependency is an internal resource
or not when looping over a resource's package.json dependencies.
*/
func SetPackageJsonsNameSet(packageJsons PackageJsons) PackageJsonsNameSet {
	result := make(map[string]bool)
	for _, packageJson := range packageJsons {
		result[packageJson.Name] = true
	}
	return result
}

type ResourceGraph struct {
	AdjacenciesMap map[string][]string
	InDegreesMap   map[string]int
	LevelsMap      map[int][]string
}

/*
NewGraph returns a resource graph.
*/
func NewGraph(resourceIdMap ResourceIDMap) *ResourceGraph {
	result := &ResourceGraph{
		AdjacenciesMap: make(map[string][]string),
		InDegreesMap:   make(map[string]int),
		LevelsMap:      make(map[int][]string),
	}

	for resourceId, resource := range resourceIdMap {
		for _, dependency := range resource.Dependencies {
			result.AddEdge(resourceId, dependency)
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

type ResourceIDToUpstreamDependenciesMap map[string][]string

/*
SetResourceIDToUpstreamDependenciesMap returns a map of resource IDs
to their upstream dependencies.

Upstream dependencies are resources that are ascendant, excluding
branches, in a keyed resource's directed acyclic graph.

For example, in a graph of A, B->A, C->B->A, D->A, D->B, X, the upstream
dependency slices are: A -> [], B -> [A], C -> [A,B], D -> [A,B], X -> [].

Inspired by:
https://www.electricmonk.nl/docs/dependency_resolving_algorithm/dependency_resolving_algorithm.html
*/
func SetResourceIDToUpstreamDependenciesMap(resourceIDMap ResourceIDMap) ResourceIDToUpstreamDependenciesMap {
	result := make(ResourceIDToUpstreamDependenciesMap)
	memo := make(map[string][]string)
	for resource := range resourceIDMap {
		result[resource] = walkDependencies(resource, resourceIDMap, memo)
	}
	return result
}

func walkDependencies(resourceID string, resourceIDMap ResourceIDMap, memo map[string][]string) []string {
	if result, found := memo[resourceID]; found {
		return result
	}

	result := make([]string, 0)
	if resourceDetail, exists := resourceIDMap[resourceID]; exists {
		dependencies := resourceDetail.Dependencies
		for _, dependency := range dependencies {
			if !helpers.IsInSlice(result, dependency) {
				result = append(result, dependency)
				for _, transitiveDependency := range walkDependencies(dependency, resourceIDMap, memo) {
					if !helpers.IsInSlice(result, transitiveDependency) {
						result = append(result, transitiveDependency)
					}
				}
			}
		}
	}
	memo[resourceID] = result

	return result
}

/*
ValidateContainerSubDirContents checks if the container subdirectories
contain the required files. The required files are: package.json in the
root, an index.ts file in the src directory, and an index.js file in the
build directory.
*/
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
