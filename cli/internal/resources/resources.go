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

/*
	[{
			ID: "core:base:cloudflare-worker:12345",
			Name: "CORE_BASE_API",
			KV: [{
				binding: "CORE_BASE_KV"
			}]
		}]
*/
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

type ResourcesUpJson ResourceIDToData

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

type HasResourceIDsToDeploy bool

func HasIDsToDeploy(stateToResourceIDs StateToResourceIDs) HasResourceIDsToDeploy {
	statesToDeploy := []State{"CREATED", "DELETED", "UPDATED"}
	for _, state := range statesToDeploy {
		if _, exists := stateToResourceIDs[state]; exists {
			return true
		}
	}
	return false
}

/*
Group 0 -> Depth 0 -> core:base:cloudflare-worker:12345
*/
func LogIDPreDeploymentStates(groupToDepthToResourceID GroupToDepthToResourceIDs, resourceIDToState ResourceIDToState) {
	fmt.Println("# Pre-Deployment States:")
	for group, depthToResourceID := range groupToDepthToResourceID {
		for depth, resourceIDs := range depthToResourceID {
			for _, resourceID := range resourceIDs {
				fmt.Printf("Group %d -> Depth %d -> %s -> %s\n", group, depth, resourceID, resourceIDToState[resourceID])
			}
		}
	}
}

type ResourceDependencyIDs [][]string

/*
[["core:base:cloudflare-kv:12345"], []]

Where index 0 is core:base:cloudflare-worker:12345's
dependency IDs and index 1 is core:base:cloudflare-kv:12345's
dependency IDs.
*/
func SetDependencyIDs(packageJsons ResourcePackageJsons, packageJsonNameToResourceIDMap PackageJsonNameToResourceID, packageJsonsNameSet PackageJsonNameToTrue) ResourceDependencyIDs {
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

type DepthToResourceID map[int][]string

/*
	{
		0: ["core:base:cloudflare-worker:12345"],
		1: ["core:base:cloudflare-kv:12345"]
	}

Depth is an int that describes how far down the graph
a resource is.

For example, given a graph of A->B, B->C, A has a depth
of 0, B has a depth of 1, and C has a depth of 2.
*/
func SetDepthToResourceID(resourceIDs ResourceIDs, resourceIDToData ResourceIDToData, resourceIDsWithInDegreesOfZero ResourceIDsWithInDegreesOf) DepthToResourceID {
	result := make(DepthToResourceID)

	numOfResourceIDsToProcess := len(resourceIDs)

	depth := 0

	for _, resourceIDWithInDegreesOfZero := range resourceIDsWithInDegreesOfZero {
		result[depth] = append(result[depth], resourceIDWithInDegreesOfZero)
		numOfResourceIDsToProcess--
	}

	for numOfResourceIDsToProcess > 0 {
		for _, resourceIDAtDepth := range result[depth] {
			for _, dependencyID := range resourceIDToData[resourceIDAtDepth].Dependencies {
				result[depth+1] = append(result[depth+1], dependencyID)
				numOfResourceIDsToProcess--
			}
		}
		depth++
	}

	return result
}

type ResourceIDToInDegrees map[string]int

/*
	{
		"core:base:cloudflare-worker:12345": 0,
		"core:base:cloudflare-kv:12345" 1
	}

In degrees is how many incoming edges a target node has.
*/
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

/*
TODO
*/
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

type GroupToHighestDeployDepth map[int]int

/*
TODO
*/
func SetGroupToHighestDeployDepth(resourceIDToDepth ResourceIDToDepth, resourceIDToState ResourceIDToState, groupsWithStateChanges GroupsWithStateChanges, groupToResourceIDs GroupToResourceIDs) GroupToHighestDeployDepth {
	result := make(GroupToHighestDeployDepth)
	for _, group := range groupsWithStateChanges {
		deployDepth := 0
		isFirstResourceToProcess := true
		for _, resourceID := range groupToResourceIDs[group] {
			// UNCHANGED resources aren't deployed, so its depth
			// can't be the deploy depth.
			if resourceIDToState[resourceID] == "UNCHANGED" {
				continue
			}

			// If resource is first to make it this far set deploy
			// depth so it can be used for comparison in future loops.
			if isFirstResourceToProcess {
				result[group] = resourceIDToDepth[resourceID]
				deployDepth = resourceIDToDepth[resourceID]
				isFirstResourceToProcess = false
				continue
			}

			// Update deploy depth if resource's depth is less than
			// the comparative deploy depth.
			if resourceIDToDepth[resourceID] < deployDepth {
				result[group] = resourceIDToDepth[resourceID]
				deployDepth = resourceIDToDepth[resourceID]
			}
		}
	}
	return result
}

type GroupToDepthToResourceIDs map[int]map[int][]string

/*
	{
		0: {
			0: ["core:base:cloudflare-worker:12345"],
			1: ["core:base:cloudflare-kv:12345"]
		},
		1: {
			0: ["admin:base:cloudflare-worker:12345"]
		}
	}
*/
func SetGroupToDepthToResourceIDs(resourceIDToGroup ResourceIDToGroup, resourceIDToDepth ResourceIDToDepth) GroupToDepthToResourceIDs {
	result := make(GroupToDepthToResourceIDs)
	for resourceID, group := range resourceIDToGroup {
		if _, exists := result[group]; !exists {
			result[group] = make(map[int][]string)
		}
		depth := resourceIDToDepth[resourceID]
		if _, exists := result[group][depth]; !exists {
			result[group][depth] = make([]string, 0)
		}
		result[group][depth] = append(result[group][depth], resourceID)
	}
	return result
}

type GroupToResourceIDs map[int][]string

/*
	{
		0: [
			"core:base:cloudflare-worker:12345", "core:base:cloudflare-kv:12345"
		],
		1: ["admin:base:cloudflare-worker:12345"]
	}
*/
func SetGroupToResourceIDs(resourceIDToGroup ResourceIDToGroup) GroupToResourceIDs {
	result := make(GroupToResourceIDs)
	for resourceID, group := range resourceIDToGroup {
		if _, exists := result[group]; !exists {
			result[group] = make([]string, 0)
		}
		result[group] = append(result[group], resourceID)
	}
	return result
}

type PackageJsonNameToResourceID map[string]string

/*
	{
		"core-base-api": "core:base:cloudflare-worker:12345"
	}
*/
func SetPackageJsonNameToID(packageJsons ResourcePackageJsons, indexBuildFileConfigs ResourceIndexBuildFileConfigs) PackageJsonNameToResourceID {
	result := make(PackageJsonNameToResourceID)
	for index, packageJson := range packageJsons {
		result[packageJson.Name] = indexBuildFileConfigs[index].ID
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

type ResourceIDs []string

/*
["core:base:cloudflare-worker:12345"]
*/
func SetIDs(resourceIDToData ResourceIDToData) ResourceIDs {
	var result ResourceIDs
	for resourceID := range resourceIDToData {
		result = append(result, resourceID)
	}
	return result
}

type ResourceIDToDeployState map[string]DeployState

type DeployState string

const (
	CANCELED           DeployState = "CANCELED"
	CREATE_COMPLETE    DeployState = "CREATE_COMPLETE"
	CREATE_FAILED      DeployState = "CREATE_FAILED"
	CREATE_IN_PROGRESS DeployState = "CREATE_IN_PROGRESS"
	DELETE_COMPLETE    DeployState = "DELETE_COMPLETE"
	DELETE_FAILED      DeployState = "DEPLOY_FAILED"
	DELETE_IN_PROGRESS DeployState = "DELETE_IN_PROGRESS"
	PENDING            DeployState = "PENDING"
	UPDATE_COMPLETE    DeployState = "UPDATE_COMPLETE"
	UPDATE_FAILED      DeployState = "UPDATE_FAILED"
	UPDATE_IN_PROGRESS DeployState = "UPDATE_IN_PROGRESS"
)

/*
	{
		"core:base:cloudflare-worker:12345": "PENDING",
	}
*/
func SetResourceIDToDeployStateOfPending(resourceIDToState ResourceIDToState) ResourceIDToDeployState {
	result := make(ResourceIDToDeployState)
	for resourceID, deployState := range resourceIDToState {
		if deployState != "UNCHANGED" {
			result[resourceID] = "PENDING"
		}
	}
	return result
}

type ResourceIDToDepth map[string]int

/*
	{
		"core:base:cloudflare-kv:12345": 1,
		"core:base:cloudflare-worker:12345": 0
	}
*/
func SetIDToDepth(depthToResourceID DepthToResourceID) ResourceIDToDepth {
	result := make(ResourceIDToDepth)
	for depth, resourceIDs := range depthToResourceID {
		for _, resourceID := range resourceIDs {
			result[resourceID] = depth
		}
	}
	return result
}

type ResourceIDToGroup map[string]int

/*
	{
		"admin:base:cloudflare-worker:12345": 0,
		"core:base:cloudflare-worker:12345": 1,
		"core:base:cloudflare-kv:12345": 1,
	}

A group is an int assigned to resource IDs that share
at least one common relative.
*/
func SetIDToGroup(resourceIDsWithInDegreesOfZero ResourceIDsWithInDegreesOf, resourceIDToIntermediateIDs ResourceIDToIntermediateIDs) ResourceIDToGroup {
	result := make(ResourceIDToGroup)
	group := 0
	for _, sourceResourceID := range resourceIDsWithInDegreesOfZero {
		if _, exists := result[sourceResourceID]; !exists {
			// Initialize source resource's group.
			result[sourceResourceID] = group

			// Set group for source resource's intermediates.
			for _, intermediateID := range resourceIDToIntermediateIDs[sourceResourceID] {
				if _, exists := result[intermediateID]; !exists {
					result[intermediateID] = group
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
			for _, possibleDistantRelativeID := range resourceIDsWithInDegreesOfZero {
				// Skip source resource from the main for loop.
				if possibleDistantRelativeID != sourceResourceID {
					// Loop over possible distant relative's intermediates.
					for _, possibleDistantRelativeIntermediateID := range resourceIDToIntermediateIDs[possibleDistantRelativeID] {
						// Check if possible distant relative's intermediate
						// is also an intermediate of source resource.
						if helpers.IncludesString(resourceIDToIntermediateIDs[sourceResourceID], possibleDistantRelativeIntermediateID) {
							// If so, possibl distant relative and source resource
							// are distant relatives and belong to the same group.
							result[possibleDistantRelativeID] = group
						}
					}
				}
			}
			group++
		}
	}
	return result
}

type ResourceIDToIntermediateIDs map[string][]string

/*
	{
		"core:base:cloudflare-worker:1235": ["core:base:cloudflare-kv:12345"],
		"core:base:cloudflare-kv:12345": []
	}

Intermediate IDs are resource IDs within the source resource's
directed path when analyzing resource relationships as a graph.

For example, given a graph of A->B, B->C, and X->C, B and C are
intermediates of A, C is an intermediate of B, and C is an
intermediate of X.

Finding intermediate IDs is necessary for grouping related resources.
It wouldn't be possible to know A and X are relatives in the above
example without them.
*/
func SetIDToIntermediateIDs(resourceIDToData ResourceIDToData) ResourceIDToIntermediateIDs {
	result := make(ResourceIDToIntermediateIDs)
	memo := make(map[string][]string)
	for resourceID := range resourceIDToData {
		result[resourceID] = walkDependencies(resourceID, resourceIDToData, memo)
	}
	return result
}

func walkDependencies(resourceID string, resourceIDToData ResourceIDToData, memo map[string][]string) []string {
	if result, found := memo[resourceID]; found {
		return result
	}

	result := make([]string, 0)
	if resourceData, exists := resourceIDToData[resourceID]; exists {
		dependencies := resourceData.Dependencies
		for _, dependency := range dependencies {
			if !helpers.IsInSlice(result, dependency) {
				result = append(result, dependency)
				for _, transitiveDependency := range walkDependencies(dependency, resourceIDToData, memo) {
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

type State string

const (
	CREATED   State = "CREATED"
	DELETED   State = "DELETED"
	UNCHANGED State = "UNCHANGED"
	UPDATED   State = "UPDATED"
)

type ResourceIDToState map[string]State

/*
TODO
*/
func SetIDToStateMap(upJson ResourcesUpJson, currResourceMap ResourceIDToData) ResourceIDToState {
	result := make(ResourceIDToState)

	for upJsonResourceID := range upJson {
		if _, exists := currResourceMap[upJsonResourceID]; !exists {
			result[upJsonResourceID] = DELETED
		}
	}

	for currResourceID, currResource := range currResourceMap {
		if _, exists := upJson[currResourceID]; !exists {
			result[currResourceID] = CREATED
		} else {
			upResource := upJson[currResourceID]
			if IsResourceEqual(upResource, currResource) {
				result[currResourceID] = UNCHANGED
			} else {
				result[currResourceID] = UPDATED
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

Where a resource of core:base:cloudflare-worker:12345
belonging to group 0 has been created.
*/
func SetGroupsWithStateChanges(resourceIDToGroup ResourceIDToGroup, resourceIDToState ResourceIDToState) GroupsWithStateChanges {
	result := make(GroupsWithStateChanges, 0)

	seenGroups := make(map[int]struct{})

	for resourceID, state := range resourceIDToState {
		if state != UNCHANGED {
			group, exists := resourceIDToGroup[resourceID]
			if exists {
				if _, alreadyAdded := seenGroups[group]; !alreadyAdded {
					result = append(result, group)
					seenGroups[group] = struct{}{}
				}
			}
		}
	}

	return result
}

type ResourceIDsWithInDegreesOf []string

/*
TODO
*/
func SetIDsWithInDegreesOf(IDToInDegrees ResourceIDToInDegrees, degrees int) ResourceIDsWithInDegreesOf {
	var result ResourceIDsWithInDegreesOf
	for resourceID, inDegree := range IDToInDegrees {
		if inDegree == degrees {
			result = append(result, resourceID)
		}
	}
	return result
}

type StateToResourceIDs = map[State][]string

/*
	{
		CREATED: ["core:base:cloudflare-worker:12345"],
		DELETED: ["..."],
		UNCHANGED: ["..."],
		UPDATED: ["..."]
	}
*/
func SetStateToResourceIDs(resourceIDToState ResourceIDToState) StateToResourceIDs {
	result := make(StateToResourceIDs)
	for resourceID, state := range resourceIDToState {
		if _, exists := result[state]; !exists {
			result[state] = make([]string, 0)
		}
		result[state] = append(result[state], resourceID)
	}
	return result
}

/*
TODO
*/
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
