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

type ContainerSubdirPaths []string

/*
["gas/core-base-api"]
*/
func GetContainerSubdirPaths(containerDir string) (ContainerSubdirPaths, error) {
	var result ContainerSubdirPaths

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

type IndexFilePaths = []string

/*
["gas/core-base-api/src/core-base-api._index.ts"]
*/
func GetIndexFilePaths(containerSubdirPaths ContainerSubdirPaths) (IndexFilePaths, error) {
	var result IndexFilePaths

	pattern := regexp.MustCompile(`^_[^.]+\.[^.]+\.[^.]+\.index\.ts$`)

	for _, subdirPath := range containerSubdirPaths {
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

/*
TypeScript type equivalent:

	Array<{
		[key: string]: any
	}>
*/
type IndexBuildFileConfigs = []map[string]interface{}

/*
	[{
			Name: "CORE_BASE_API",
			KV: [{
				binding: "CORE_BASE_KV"
			}]
		}]
*/
func GetIndexBuildFileConfigs(containerSubdirPaths ContainerSubdirPaths, indexFilePaths IndexFilePaths, indexBuildFilePaths indexBuildFilePaths) (IndexBuildFileConfigs, error) {
	var result IndexBuildFileConfigs

	embedPath := "embed/get-index-build-file-configs.js"

	content, err := getIndexBuildFileConfigsEmbed.ReadFile(embedPath)
	if err != nil {
		return result, fmt.Errorf("unable to read embed %s", embedPath)
	}

	nodeCmd := exec.Command("node", "--input-type=module")

	subdirPaths := strings.Join(containerSubdirPaths, ",")
	nodeCmd.Env = append(nodeCmd.Env, "RESOURCE_CONTAINER_SUBDIR_PATHS="+subdirPaths)

	filePaths := strings.Join(indexFilePaths, ",")
	nodeCmd.Env = append(nodeCmd.Env, "RESOURCE_INDEX_FILE_PATHS="+filePaths)

	buildFilePaths := strings.Join(indexBuildFilePaths, ",")
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

type indexBuildFilePaths = []string

/*
["gas/core-base-api/build/core-base-api._index.js"]
*/
func GetIndexBuildFilePaths(containerSubdirPaths ContainerSubdirPaths) (indexBuildFilePaths, error) {
	var result indexBuildFilePaths

	pattern := regexp.MustCompile(`^_[^.]+\.[^.]+\.[^.]+\.index\.js$`)

	for _, subdirPath := range containerSubdirPaths {
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

type PackageJsons []PackageJson

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
func GetPackageJsons(containerSubdirPaths ContainerSubdirPaths) (PackageJsons, error) {
	var result PackageJsons

	for _, subdirPath := range containerSubdirPaths {
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

type UpJsonNew map[string]interface{}

func GetUpJsonNew(filePath string) (UpJsonNew, error) {
	var result UpJsonNew

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("unable to read up .json file %s\n%v", filePath, err)
	}

	err = json.Unmarshal(data, &result)
	if err != nil {
		return nil, fmt.Errorf("unable to parse up .json file %s\n%v", filePath, err)
	}

	return result, nil
}

type UpJson NameToData

/*
TODO
*/
func GetUpJson(resourcesUpJsonPath string) (UpJson, error) {
	var result UpJson
	err := helpers.UnmarshallFile(resourcesUpJsonPath, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

type DependencyNames [][]string

/*
[["CORE_BASE_KV"], []]

Where index 0 is CORE_BASE_API's dependency names and index 1
is CORE_BASE_KV's dependency names.
*/
func SetDependencyNames(packageJsons PackageJsons, packageJsonNameToNameMap PackageJsonNameToName, packageJsonsNameSet PackageJsonNameToTrue) DependencyNames {
	var result DependencyNames
	for _, packageJson := range packageJsons {
		var internalDependencyNames []string
		for dependencyName := range packageJson.Dependencies {
			resourceName, ok := packageJsonNameToNameMap[dependencyName]
			if ok && packageJsonsNameSet[dependencyName] {
				internalDependencyNames = append(internalDependencyNames, resourceName)
			}
		}
		result = append(result, internalDependencyNames)
	}
	return result
}

type DepthToName map[int][]string

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
func SetDepthToName(names Names, nameToDependencies NameToDependencies, namesWithInDegreesOfZero namesWithInDegreesOf) DepthToName {
	result := make(DepthToName)

	numOfNamesToProcess := len(names)

	depth := 0

	for _, nameWithInDegreesOfZero := range namesWithInDegreesOfZero {
		result[depth] = append(result[depth], nameWithInDegreesOfZero)
		numOfNamesToProcess--
	}

	for numOfNamesToProcess > 0 {
		for _, nameAtDepth := range result[depth] {
			for _, dependencyName := range nameToDependencies[nameAtDepth] {
				result[depth+1] = append(result[depth+1], dependencyName)
				numOfNamesToProcess--
			}
		}
		depth++
	}

	return result
}

type NameToInDegrees map[string]int

/*
	{
		"CORE_BASE_API": 0,
		"CORE_BASE_KV" 1
	}

In degrees is how many incoming edges a target resource has.
*/
func SetNameToInDegrees(nameToDependencies NameToDependencies) NameToInDegrees {
	result := make(NameToInDegrees)

	// Loop over resources and their dependencies.
	for _, dependencies := range nameToDependencies {
		// Increment resource's in degrees everytime it's
		// found to be a dependency of another resource.
		for _, dependencyName := range dependencies {
			result[dependencyName]++
		}
	}

	for name := range nameToDependencies {
		// Resource has to have an in degrees of 0 if it
		// isn't in the result.
		if _, ok := result[name]; !ok {
			result[name] = 0
		}
	}

	return result
}

type NameToData map[string]Resource

type Resource struct {
	Type         string
	Config       interface{}
	Dependencies []string
}

/*
TODO
*/
func SetNameToData(indexBuildFileConfigs IndexBuildFileConfigs, dependencyNames DependencyNames) NameToData {
	result := make(NameToData)
	for index := range indexBuildFileConfigs {
		result["CORE_BASE_KV"] = Resource{
			Type: "cloudflare-kv",
			Config: &CloudflareKVConfig{
				Type: "cloudflare-kv",
				Name: "CORE_BASE_KV",
			},
			Dependencies: dependencyNames[index],
		}
	}
	return result
}

type NameToConfig map[string]interface{}

/*
TODO
*/
func SetNameToConfig(indexBuildFileConfigs IndexBuildFileConfigs) NameToConfig {
	result := make(NameToConfig)
	for _, config := range indexBuildFileConfigs {
		resourceType := config["type"].(string)
		name := config["name"].(string)
		result[name] = configs[resourceType](config)
	}
	return result
}

var configs = map[string]func(config config) interface{}{
	"cloudflare-kv": func(config config) interface{} {
		return &CloudflareKVConfig{
			Type: config["type"].(string),
			Name: config["name"].(string),
		}
	},
	"cloudflare-worker": func(config config) interface{} {
		return &CloudflareWorkerConfig{}
	},
}

/*
TypeScript type equivalent:

	Array<{
		[key: string]: any
	}>
*/
type config map[string]interface{}

type ConfigCommon struct {
	Type string
	Name string
}

type CloudflareKVConfig struct {
	Type string
	Name string
}

type CloudflareWorkerConfig struct {
	Type string
	Name string
	KV   []struct {
		Binding string
	}
}

type NameToDependencies map[string][]string

/*
TODO
*/
func SetNameToDependencies(indexBuildFileConfigs IndexBuildFileConfigs, dependencyNames DependencyNames) NameToDependencies {
	result := make(NameToDependencies)
	for index, config := range indexBuildFileConfigs {
		name := config["name"].(string)
		result[name] = dependencyNames[index]
	}
	return result
}

type GroupToHighestDeployDepth map[int]int

/*
TODO
*/
func SetGroupToHighestDeployDepth(nameToDepth NameToDepth, nameToState NameToState, groupsWithStateChanges GroupsWithStateChanges, groupToNames GroupToNames) GroupToHighestDeployDepth {
	result := make(GroupToHighestDeployDepth)
	for _, group := range groupsWithStateChanges {
		deployDepth := 0
		isFirstResourceToProcess := true
		for _, name := range groupToNames[group] {
			// UNCHANGED resources aren't deployed, so its depth
			// can't be the deploy depth.
			if nameToState[name] == State("UNCHANGED") {
				continue
			}

			// If resource is first to make it this far set deploy
			// depth so it can be used for comparison in future loops.
			if isFirstResourceToProcess {
				result[group] = nameToDepth[name]
				deployDepth = nameToDepth[name]
				isFirstResourceToProcess = false
				continue
			}

			// Update deploy depth if resource's depth is greater than
			// the comparative deploy depth.
			if nameToDepth[name] > deployDepth {
				result[group] = nameToDepth[name]
				deployDepth = nameToDepth[name]
			}
		}
	}
	return result
}

type GroupToDepthToNames map[int]map[int][]string

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
func SetGroupToDepthToNames(nameToGroup NameToGroup, nameToDepth NameToDepth) GroupToDepthToNames {
	result := make(GroupToDepthToNames)
	for name, group := range nameToGroup {
		if _, ok := result[group]; !ok {
			result[group] = make(map[int][]string)
		}
		depth := nameToDepth[name]
		if _, ok := result[group][depth]; !ok {
			result[group][depth] = make([]string, 0)
		}
		result[group][depth] = append(result[group][depth], name)
	}
	return result
}

type GroupToNames map[int][]string

/*
	{
		0: [
			"CORE_BASE_API", "CORE_BASE_KV"
		],
		1: ["ADMIN_BASE_API"]
	}
*/
func SetGroupToNames(nameToGroup NameToGroup) GroupToNames {
	result := make(GroupToNames)
	for name, group := range nameToGroup {
		if _, ok := result[group]; !ok {
			result[group] = make([]string, 0)
		}
		result[group] = append(result[group], name)
	}
	return result
}

type PackageJsonNameToName map[string]string

/*
	{
		"core-base-api": "CORE_BASE_API"
	}
*/
func SetPackageJsonNameToName(packageJsons PackageJsons, indexBuildFileConfigs IndexBuildFileConfigs) PackageJsonNameToName {
	result := make(PackageJsonNameToName)
	for index, packageJson := range packageJsons {
		resourceName := indexBuildFileConfigs[index]["name"].(string)
		result[packageJson.Name] = resourceName
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
func SetPackageJsonNameToTrue(packageJsons PackageJsons) PackageJsonNameToTrue {
	result := make(PackageJsonNameToTrue)
	for _, packageJson := range packageJsons {
		result[packageJson.Name] = true
	}
	return result
}

type Names []string

/*
["CORE_BASE_API"]
*/
func SetNames(nameToData NameToData) Names {
	var result Names
	for name := range nameToData {
		result = append(result, name)
	}
	return result
}

type NameToDepth map[string]int

/*
	{
		"CORE_BASE_KV": 1,
		"CORE_BASE_API": 0
	}
*/
func SetNameToDepth(depthToName DepthToName) NameToDepth {
	result := make(NameToDepth)
	for depth, names := range depthToName {
		for _, name := range names {
			result[name] = depth
		}
	}
	return result
}

type NameToGroup map[string]int

/*
	{
		"ADMIN_BASE_API": 0,
		"CORE_BASE_API": 1,
		"CORE_BASE_KV": 1,
	}

A group is an int assigned to resource names that share
at least one common relative.
*/
func SetNameToGroup(namesWithInDegreesOfZero namesWithInDegreesOf, nameToIntermediateNames NameToIntermediateNames) NameToGroup {
	result := make(NameToGroup)
	group := 0
	for _, sourceName := range namesWithInDegreesOfZero {
		if _, ok := result[sourceName]; !ok {
			// Initialize source resource's group.
			result[sourceName] = group

			// Set group for source resource's intermediates.
			for _, intermediateName := range nameToIntermediateNames[sourceName] {
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
			for _, possibleDistantRelativeName := range namesWithInDegreesOfZero {
				// Skip source resource from the main for loop.
				if possibleDistantRelativeName != sourceName {
					// Loop over possible distant relative's intermediates.
					for _, possibleDistantRelativeIntermediateName := range nameToIntermediateNames[possibleDistantRelativeName] {
						// Check if possible distant relative's intermediate
						// is also an intermediate of source resource.
						if helpers.IncludesString(nameToIntermediateNames[sourceName], possibleDistantRelativeIntermediateName) {
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

type NameToIntermediateNames map[string][]string

/*
	{
		"CORE_BASE_API": ["CORE_BASE_KV"],
		"CORE_BASE_KV": []
	}

Intermediate names are names within the source resource's
directed path when analyzing resource relationships as a graph.

For example, given a graph of A->B, B->C, and X->C, B and C are
intermediates of A, C is an intermediate of B, and C is an
intermediate of X.

Finding intermediate names is necessary for grouping related resources.
It wouldn't be possible to know A and X are relatives in the above
example without them.
*/
func SetNameToIntermediateNames(nameToDependencies NameToDependencies) NameToIntermediateNames {
	result := make(NameToIntermediateNames)
	memo := make(map[string][]string)
	for name := range nameToDependencies {
		result[name] = walkDependencies(name, nameToDependencies, memo)
	}
	return result
}

func walkDependencies(name string, nameToDependencies NameToDependencies, memo map[string][]string) []string {
	if result, found := memo[name]; found {
		return result
	}

	result := make([]string, 0)
	if dependencyNames, ok := nameToDependencies[name]; ok {
		for _, dependencyName := range dependencyNames {
			if !helpers.IsInSlice(result, dependencyName) {
				result = append(result, dependencyName)
				for _, transitiveDependency := range walkDependencies(dependencyName, nameToDependencies, memo) {
					if !helpers.IsInSlice(result, transitiveDependency) {
						result = append(result, transitiveDependency)
					}
				}
			}
		}
	}
	memo[name] = result

	return result
}

type State string

const (
	CREATED   State = "CREATED"
	DELETED   State = "DELETED"
	UNCHANGED State = "UNCHANGED"
	UPDATED   State = "UPDATED"
)

type NameToState map[string]State

func SetNameToStateMapNew() {}

func isEqualNew(upConfig map[string]interface{}, upNameToDependencies UpNameToDependencies, resourceConfig map[string]interface{}, resourceNameToDependencies NameToDependencies) bool {
	upConfigType := upConfig["type"].(string)
	resource2ConfigType := resourceConfig["type"].(string)

	// This probably shouldn't even happen?
	if upConfigType != resource2ConfigType {
		return false
	}

	if !reflect.DeepEqual(upConfig, resourceConfig) {
		return false
	}

	if !reflect.DeepEqual(upNameToDependencies, resourceNameToDependencies) {
		return false
	}

	return true
}

/*
TODO
*/
func SetNameToStateMap(upJson UpJson, nameToData NameToData) NameToState {
	result := make(NameToState)

	for name := range upJson {
		if _, ok := nameToData[name]; !ok {
			result[name] = State(DELETED)
		}
	}

	for name, resource := range nameToData {
		if _, ok := upJson[name]; !ok {
			result[name] = State(CREATED)
		} else {
			upResource := upJson[name]
			if isEqual(upResource, resource) {
				result[name] = State(UNCHANGED)
			} else {
				result[name] = State(UPDATED)
			}
		}
	}

	return result
}

/*
TODO
*/
func isEqual(resource1, resource2 Resource) bool {
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
func SetGroupsWithStateChanges(nameToGroup NameToGroup, nameToState NameToState) GroupsWithStateChanges {
	result := make(GroupsWithStateChanges, 0)

	seenGroups := make(map[int]struct{})

	for name, state := range nameToState {
		if state != UNCHANGED {
			group, ok := nameToGroup[name]
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

type namesWithInDegreesOf []string

/*
TODO
*/
func SetNamesWithInDegreesOf(nameToInDegrees NameToInDegrees, degrees int) namesWithInDegreesOf {
	var result namesWithInDegreesOf
	for name, inDegree := range nameToInDegrees {
		if inDegree == degrees {
			result = append(result, name)
		}
	}
	return result
}

type StateToNames = map[State][]string

/*
	{
		CREATED: ["core:base:cloudflare-worker:12345"],
		DELETED: ["..."],
		UNCHANGED: ["..."],
		UPDATED: ["..."]
	}
*/
func SetStateToNames(nameToState NameToState) StateToNames {
	result := make(StateToNames)
	for name, state := range nameToState {
		if _, ok := result[state]; !ok {
			result[state] = make([]string, 0)
		}
		result[state] = append(result[state], name)
	}
	return result
}

type UpNameToConfig map[string]interface{}

type UpNameToDependencies map[string][]string

func SetUpNameToDependencies(upJson UpJsonNew) UpNameToDependencies {
	result := make(UpNameToDependencies)
	for name, data := range upJson {
		dependenciesInterface := data.(map[string]interface{})["dependencies"]
		if dependenciesInterface != nil {
			dependenciesSlice := dependenciesInterface.([]interface{})
			dependencies := make([]string, len(dependenciesSlice))
			for index, dependency := range dependenciesSlice {
				dependencies[index] = dependency.(string)
			}
			result[name] = dependencies
		} else {
			result[name] = []string{}
		}
	}
	return result
}
