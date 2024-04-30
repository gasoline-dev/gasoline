package resources

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
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

/*
["gas/core-base-api"]
*/
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

/*
["gas/core-base-api/src/core-base-api._index.ts"]
*/
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
			if !file.IsDir() && pattern.MatchString(file.Name()) {
				result = append(result, filepath.Join(srcPath, file.Name()))
			}
		}
	}

	return result, nil
}

type ResourceIndexBuildFileConfigs = []Config

type Config struct {
	Name string `json:"name"`
	KV   []struct {
		Binding string `json:"binding"`
	} `json:"kv,omitempty"`
}

/*
	[{
			Name: "CORE_BASE_API",
			KV: [{
				binding: "CORE_BASE_KV"
			}]
		}]
*/
func GetIndexBuildFileConfigs(resourceContainerSubdirPaths ResourceContainerSubdirPaths, resourceIndexFilePaths ResourceIndexFilePaths, resourceIndexBuildFilePaths ResourceIndexBuildFilePaths) (ResourceIndexBuildFileConfigs, error) {
	var result ResourceIndexBuildFileConfigs

	embedPath := "embed/get-index-build-file-configs.js"

	content, err := getIndexBuildFileConfigsEmbed.ReadFile(embedPath)
	if err != nil {
		return result, fmt.Errorf("unable to read embed %s", embedPath)
	}

	nodeCmd := exec.Command("node", "--input-type=module")

	subdirPaths := strings.Join(resourceContainerSubdirPaths, ",")
	nodeCmd.Env = append(nodeCmd.Env, "RESOURCE_CONTAINER_SUBDIR_PATHS="+subdirPaths)

	filePaths := strings.Join(resourceIndexFilePaths, ",")
	nodeCmd.Env = append(nodeCmd.Env, "RESOURCE_INDEX_FILE_PATHS="+filePaths)

	buildFilePaths := strings.Join(resourceIndexBuildFilePaths, ",")
	nodeCmd.Env = append(nodeCmd.Env, "RESOURCE_INDEX_BUILD_FILE_PATHS="+buildFilePaths)

	nodeCmd.Stdin = bytes.NewReader(content)

	output, err := nodeCmd.CombinedOutput()
	if err != nil {
		return result, fmt.Errorf("unable to execute embed %s\n%s", embedPath, output)
	}

	strOutput := strings.TrimSpace((string(output)))

	if strings.Contains(strOutput, "Error:") {
		return result, errors.New("unable to process exported resource configs")
	}

	err = json.Unmarshal([]byte(strOutput), &result)
	if err != nil {
		return result, fmt.Errorf("unable to parse exported resource configs JSON result\n%v", err)
	}

	return result, nil
}

type ResourceIndexBuildFilePaths = []string

/*
["gas/core-base-api/build/core-base-api._index.js"]
*/
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

/*
TODO

	[{
		Name: "",
		Main: "",
		Types: "",
		Scripts: "",
		Dependencies: {},
		DevDependencies: {}
	}]
*/
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

type ResourcesUpJson ResourceNameToData

/*
TODO
*/
func GetUpJson(resourcesUpJsonPath string) (ResourcesUpJson, error) {
	var result ResourcesUpJson
	err := helpers.UnmarshallFile(resourcesUpJsonPath, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

type ResourceDependencyNames [][]string

/*
[["CORE_BASE_KV"], []]

Where index 0 is CORE_BASE_API's dependency names and index 1
is CORE_BASE_KV's dependency names.
*/
func SetDependencyNames(packageJsons ResourcePackageJsons, packageJsonNameToResourceNameMap PackageJsonNameToResourceName, packageJsonsNameSet PackageJsonNameToTrue) ResourceDependencyNames {
	var result ResourceDependencyNames
	for _, packageJson := range packageJsons {
		var internalDependencies []string
		for dependency := range packageJson.Dependencies {
			resourceName, ok := packageJsonNameToResourceNameMap[dependency]
			if ok && packageJsonsNameSet[dependency] {
				internalDependencies = append(internalDependencies, resourceName)
			}
		}
		result = append(result, internalDependencies)
	}
	return result
}

type DepthToResourceName map[int][]string

/*
	{
		0: ["CORE_BASE_API"],
		1: ["CORE_BASE_KV"]
	}

Depth is an int that describes how far down the graph
a resource is.

For example, given a graph of A->B, B->C, A has a depth
of 0, B has a depth of 1, and C has a depth of 2.
*/
func SetDepthToResourceName(resourceNames ResourceNames, resourceNameToData ResourceNameToData, resourceNamesWithInDegreesOfZero ResourceNamesWithInDegreesOf) DepthToResourceName {
	result := make(DepthToResourceName)

	numOfResourceNamesToProcess := len(resourceNames)

	depth := 0

	for _, resourceNameWithInDegreesOfZero := range resourceNamesWithInDegreesOfZero {
		result[depth] = append(result[depth], resourceNameWithInDegreesOfZero)
		numOfResourceNamesToProcess--
	}

	for numOfResourceNamesToProcess > 0 {
		for _, resourceNameAtDepth := range result[depth] {
			for _, dependencyName := range resourceNameToData[resourceNameAtDepth].Dependencies {
				result[depth+1] = append(result[depth+1], dependencyName)
				numOfResourceNamesToProcess--
			}
		}
		depth++
	}

	return result
}

type ResourceNameToInDegrees map[string]int

/*
	{
		"CORE_BASE_API": 0,
		"CORE_BASE_KV" 1
	}

In degrees is how many incoming edges a target node has.
*/
func SetNameToInDegrees(resourceMap ResourceNameToData) ResourceNameToInDegrees {
	result := make(ResourceNameToInDegrees)
	for _, resource := range resourceMap {
		for _, resourceDependencyName := range resource.Dependencies {
			result[resourceDependencyName]++
		}
	}
	for resourceName := range resourceMap {
		if _, ok := result[resourceName]; !ok {
			result[resourceName] = 0
		}
	}
	return result
}

type ResourceNameToData map[string]Resource

type Resource struct {
	Type         string
	Config       Config
	Dependencies []string
}

type ResourceConfig interface {
	GetValues() map[string]any
}

type CloudflareWorker struct {
	Type string
	KV   []struct {
		Binding string `json:"binding"`
	} `json:"kv,omitempty"`
}

func (cloudflareWorker CloudflareWorker) GetValues() map[string]any {
	return map[string]any{"Type": cloudflareWorker.Type, "X": cloudflareWorker.KV}
}

type CloudflareKV struct {
	Type string
	Idk  string
}

func (cloudflareKV CloudflareKV) GetValues() map[string]any {
	return map[string]any{"Type": cloudflareKV.Type, "X": cloudflareKV.Idk}
}

/*
TODO
*/
func SetNameToData(indexBuildFileConfigs ResourceIndexBuildFileConfigs, resourceDependencyNames ResourceDependencyNames) ResourceNameToData {
	result := make(ResourceNameToData)
	for index, config := range indexBuildFileConfigs {
		result[config.Name] = Resource{
			Type:         "cloudflare-kv",
			Config:       config,
			Dependencies: resourceDependencyNames[index],
		}
	}
	return result
}

type GroupToHighestDeployDepth map[int]int

/*
TODO
*/
func SetGroupToHighestDeployDepth(resourceNameToDepth ResourceNameToDepth, resourceNameToState ResourceNameToState, groupsWithStateChanges GroupsWithStateChanges, groupToResourceNames GroupToResourceNames) GroupToHighestDeployDepth {
	result := make(GroupToHighestDeployDepth)
	for _, group := range groupsWithStateChanges {
		deployDepth := 0
		isFirstResourceToProcess := true
		for _, resourceName := range groupToResourceNames[group] {
			// UNCHANGED resources aren't deployed, so its depth
			// can't be the deploy depth.
			if resourceNameToState[resourceName] == State("UNCHANGED") {
				continue
			}

			// If resource is first to make it this far set deploy
			// depth so it can be used for comparison in future loops.
			if isFirstResourceToProcess {
				result[group] = resourceNameToDepth[resourceName]
				deployDepth = resourceNameToDepth[resourceName]
				isFirstResourceToProcess = false
				continue
			}

			// Update deploy depth if resource's depth is greater than
			// the comparative deploy depth.
			if resourceNameToDepth[resourceName] > deployDepth {
				result[group] = resourceNameToDepth[resourceName]
				deployDepth = resourceNameToDepth[resourceName]
			}
		}
	}
	return result
}

type GroupToDepthToResourceNames map[int]map[int][]string

/*
	{
		0: {
			0: ["CORE_BASE_API"],
			1: ["CORE_BASE_KV"]
		},
		1: {
			0: ["ADMIN_BASE_API"]
		}
	}
*/
func SetGroupToDepthToResourceNames(resourceNameToGroup ResourceNameToGroup, resourceNameToDepth ResourceNameToDepth) GroupToDepthToResourceNames {
	result := make(GroupToDepthToResourceNames)
	for resourceName, group := range resourceNameToGroup {
		if _, ok := result[group]; !ok {
			result[group] = make(map[int][]string)
		}
		depth := resourceNameToDepth[resourceName]
		if _, ok := result[group][depth]; !ok {
			result[group][depth] = make([]string, 0)
		}
		result[group][depth] = append(result[group][depth], resourceName)
	}
	return result
}

type GroupToResourceNames map[int][]string

/*
	{
		0: [
			"CORE_BASE_API", "CORE_BASE_KV"
		],
		1: ["ADMIN_BASE_API"]
	}
*/
func SetGroupToResourceNames(resourceNameToGroup ResourceNameToGroup) GroupToResourceNames {
	result := make(GroupToResourceNames)
	for resourceName, group := range resourceNameToGroup {
		if _, ok := result[group]; !ok {
			result[group] = make([]string, 0)
		}
		result[group] = append(result[group], resourceName)
	}
	return result
}

type PackageJsonNameToResourceName map[string]string

/*
	{
		"core-base-api": "CORE_BASE_API"
	}
*/
func SetPackageJsonNameToResourceName(packageJsons ResourcePackageJsons, indexBuildFileConfigs ResourceIndexBuildFileConfigs) PackageJsonNameToResourceName {
	result := make(PackageJsonNameToResourceName)
	for index, packageJson := range packageJsons {
		result[packageJson.Name] = indexBuildFileConfigs[index].Name
	}
	return result
}

type PackageJsonNameToTrue map[string]bool

/*
	{
		"core-base-api": true,
		"core-base-kv": true
	}

This map can be used to tell if a dependency is an internal
resource or not when looping over a resource's package.json
dependencies.

For example, given a relationship of CORE_BASE_API -> CORE_BASE_KV,
when looping over CORE_BASE_API's package.json dependencies, each
dependency can be checked against this map. If a check returns true,
then the dependency is another resource.
*/
func SetPackageJsonNameToTrue(packageJsons ResourcePackageJsons) PackageJsonNameToTrue {
	result := make(PackageJsonNameToTrue)
	for _, packageJson := range packageJsons {
		result[packageJson.Name] = true
	}
	return result
}

type ResourceNames []string

/*
["CORE_BASE_API"]
*/
func SetNames(resourceNameToData ResourceNameToData) ResourceNames {
	var result ResourceNames
	for resourceName := range resourceNameToData {
		result = append(result, resourceName)
	}
	return result
}

type ResourceNameToDepth map[string]int

/*
	{
		"CORE_BASE_KV": 1,
		"CORE_BASE_API": 0
	}
*/
func SetNameToDepth(depthToResourceName DepthToResourceName) ResourceNameToDepth {
	result := make(ResourceNameToDepth)
	for depth, resourceNames := range depthToResourceName {
		for _, resourceName := range resourceNames {
			result[resourceName] = depth
		}
	}
	return result
}

type ResourceNameToGroup map[string]int

/*
	{
		"ADMIN_BASE_API": 0,
		"CORE_BASE_API": 1,
		"CORE_BASE_KV": 1,
	}

A group is an int assigned to resource names that share
at least one common relative.
*/
func SetNameToGroup(resourceNamesWithInDegreesOfZero ResourceNamesWithInDegreesOf, resourceNameToIntermediateNames ResourceNameToIntermediateNames) ResourceNameToGroup {
	result := make(ResourceNameToGroup)
	group := 0
	for _, sourceResourceName := range resourceNamesWithInDegreesOfZero {
		if _, ok := result[sourceResourceName]; !ok {
			// Initialize source resource's group.
			result[sourceResourceName] = group

			// Set group for source resource's intermediates.
			for _, intermediateName := range resourceNameToIntermediateNames[sourceResourceName] {
				if _, ok := result[intermediateName]; !ok {
					result[intermediateName] = group
				}
			}

			// Set group for distant relatives of source resource.
			// For example, given a graph of A->B, B->C, & X->C,
			// A & X both have an in degrees of 0. When walking the graph
			// downward from their positions, neither will gain knowledge of the
			// other's existence because they don't have incoming edges. To account
			// for that, all resources with an in degrees of 0 need to be checked
			// with one another to see if they have a common relative (common
			// intermediate resources in each's direct path). In this case, A & X
			// share a common relative in "C". Therefore, A & X should be assigned
			// to the same group.
			for _, possibleDistantRelativeName := range resourceNamesWithInDegreesOfZero {
				// Skip source resource from the main for loop.
				if possibleDistantRelativeName != sourceResourceName {
					// Loop over possible distant relative's intermediates.
					for _, possibleDistantRelativeIntermediateName := range resourceNameToIntermediateNames[possibleDistantRelativeName] {
						// Check if possible distant relative's intermediate
						// is also an intermediate of source resource.
						if helpers.IncludesString(resourceNameToIntermediateNames[sourceResourceName], possibleDistantRelativeIntermediateName) {
							// If so, possibl distant relative and source resource
							// are distant relatives and belong to the same group.
							result[possibleDistantRelativeName] = group
						}
					}
				}
			}
			group++
		}
	}
	return result
}

type ResourceNameToIntermediateNames map[string][]string

/*
	{
		"CORE_BASE_API": ["CORE_BASE_KV"],
		"CORE_BASE_KV": []
	}

Intermediate names are resource names within the source resource's
directed path when analyzing resource relationships as a graph.

For example, given a graph of A->B, B->C, and X->C, B and C are
intermediates of A, C is an intermediate of B, and C is an
intermediate of X.

Finding intermediate names is necessary for grouping related resources.
It wouldn't be possible to know A and X are relatives in the above
example without them.
*/
func SetNameToIntermediateNames(resourceNameToData ResourceNameToData) ResourceNameToIntermediateNames {
	result := make(ResourceNameToIntermediateNames)
	memo := make(map[string][]string)
	for resourceName := range resourceNameToData {
		result[resourceName] = walkDependencies(resourceName, resourceNameToData, memo)
	}
	return result
}

func walkDependencies(resourceName string, resourceNameToData ResourceNameToData, memo map[string][]string) []string {
	if result, found := memo[resourceName]; found {
		return result
	}

	result := make([]string, 0)
	if resourceData, ok := resourceNameToData[resourceName]; ok {
		resourceDependencyNames := resourceData.Dependencies
		for _, resourceDependencyName := range resourceDependencyNames {
			if !helpers.IsInSlice(result, resourceDependencyName) {
				result = append(result, resourceDependencyName)
				for _, transitiveDependency := range walkDependencies(resourceDependencyName, resourceNameToData, memo) {
					if !helpers.IsInSlice(result, transitiveDependency) {
						result = append(result, transitiveDependency)
					}
				}
			}
		}
	}
	memo[resourceName] = result

	return result
}

type State string

const (
	CREATED   State = "CREATED"
	DELETED   State = "DELETED"
	UNCHANGED State = "UNCHANGED"
	UPDATED   State = "UPDATED"
)

type ResourceNameToState map[string]State

/*
TODO
*/
func SetNameToStateMap(upJson ResourcesUpJson, currResourceMap ResourceNameToData) ResourceNameToState {
	result := make(ResourceNameToState)

	for upJsonResourceName := range upJson {
		if _, ok := currResourceMap[upJsonResourceName]; !ok {
			result[upJsonResourceName] = State(DELETED)
		}
	}

	for currResourceName, currResource := range currResourceMap {
		if _, ok := upJson[currResourceName]; !ok {
			result[currResourceName] = State(CREATED)
		} else {
			upResource := upJson[currResourceName]
			if IsResourceEqual(upResource, currResource) {
				result[currResourceName] = State(UNCHANGED)
			} else {
				result[currResourceName] = State(UPDATED)
			}
		}
	}

	return result
}

/*
TODO
*/
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

type GroupsWithStateChanges = []int

/*
[0]

Where a resource of CORE_BASE_API belonging to group 0 has been created.
*/
func SetGroupsWithStateChanges(resourceNameToGroup ResourceNameToGroup, resourceNameToState ResourceNameToState) GroupsWithStateChanges {
	result := make(GroupsWithStateChanges, 0)

	seenGroups := make(map[int]struct{})

	for resourceName, state := range resourceNameToState {
		if state != UNCHANGED {
			group, ok := resourceNameToGroup[resourceName]
			if ok {
				if _, alreadyAdded := seenGroups[group]; !alreadyAdded {
					result = append(result, group)
					seenGroups[group] = struct{}{}
				}
			}
		}
	}

	return result
}

type ResourceNamesWithInDegreesOf []string

/*
TODO
*/
func SetNamesWithInDegreesOf(resourceNameToInDegrees ResourceNameToInDegrees, degrees int) ResourceNamesWithInDegreesOf {
	var result ResourceNamesWithInDegreesOf
	for resourceName, inDegree := range resourceNameToInDegrees {
		if inDegree == degrees {
			result = append(result, resourceName)
		}
	}
	return result
}

type StateToResourceNames = map[State][]string

/*
	{
		CREATED: ["core:base:cloudflare-worker:12345"],
		DELETED: ["..."],
		UNCHANGED: ["..."],
		UPDATED: ["..."]
	}
*/
func SetStateToResourceNames(resourceNameToState ResourceNameToState) StateToResourceNames {
	result := make(StateToResourceNames)
	for resourceName, state := range resourceNameToState {
		if _, ok := result[state]; !ok {
			result[state] = make([]string, 0)
		}
		result[state] = append(result[state], resourceName)
	}
	return result
}
