package resources

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"gas/graph"
	"gas/helpers"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/spf13/viper"
)

type Resources struct {
	containerDir                string
	containerSubdirPaths        containerSubdirPaths
	nameToPackageJson           nameToPackageJson
	packageJsonNameToName       packageJsonNameToName
	nameToDeps                  nameToDeps
	nameToIndexFilePath         nameToIndexFilePath
	nameToIndexFileContent      nameToIndexFileContent
	nameToConfigData            nameToConfigData
	groupToDepthToNames         graph.GroupToDepthToNodes
	namesWithInDegreesOfZero    graph.NodesWithInDegreesOfZero
	nameToIntermediates         graph.NodeToIntermediates
	depthToName                 graph.DepthToNode
	nameToDepth                 graph.NodeToDepth
	nodeJsConfigScript          nodeJsConfigScript
	runNodeJsConfigScriptResult runNodeJsConfigScriptResult
	nameToConfig                nameToConfig
	upJsonPath                  string
	upJson                      upJson
	upNameToDeps                upNameToDeps
	upNameToConfig              upNameToConfig
	upNameToOutput              upNameToOutput
	nameToGroup                 nameToGroup
	groupsWithStateChanges      groupsWithStateChanges
	groupToNames                groupToNames
	groupToHighestDeployDepth   groupToHighestDeployDepth
	nameToState                 nameToState
	nameToDeployStateContainer  *nameToDeployStateContainer
	nameToDeployOutputContainer *nameToDeployOutputContainer
}

func New() *Resources {
	r := &Resources{}
	return r
}

/*
Resources can be derived from the resource container
dir and up .json file.

Resources derived from the resource container dir are
considered as being current (curr) resources. They're a
snapshot of the system's resources as they currently
exist locally.

Resources derived from the up .json file are a mixture of
current and past resources -- depending on what changes have
or haven't been made. They're a snapshot of the system's
resources on last deploy to the cloud.

The init of current resource values is split in parts because
a merge of currrent resource deps and up resource deps has to
happen before setting the resource graph (in the context of
the "up" command)*.

Additionally, the resource graph has to be set before setting
the Node.js config script because the script has to write the
configs in bottom-up dependency order (see setNodeJsConfigScript).

Summary: unlike initUp(), init current funcs have to be split up
because a merge between current and up .json file resource has to
happen before setting the graph, and the graph has to be set before
setting the Node.js config script. Therefore, there can't be one
initCurr() func.

* The merge has to happen because the up .json file may have
resources that don't exist in the current resource maps
because the resources were deleted. Those deleted resources
are accounted for by merging current and up resource to deps
maps. Only then is the resource graph complete.
*/
func (r *Resources) InitWithUp() error {
	err := r.initUp()
	if err != nil {
		return err
	}

	err = r.initPreParseConfigCurr()
	if err != nil {
		return err
	}

	r.nameToDeps = helpers.MergeStringSliceMaps(r.upNameToDeps, r.nameToDeps)

	g := graph.New(graph.NodeToDeps(r.nameToDeps))

	r.groupToDepthToNames = g.GroupToDepthToNodes

	r.namesWithInDegreesOfZero = g.NodesWithInDegreesOfZero

	r.nameToIntermediates = g.NodeToIntermediates

	r.depthToName = g.DepthToNode

	r.nameToDepth = g.NodeToDepth

	r.initParseConfigCurr()

	r.initPostConfigCurr()

	r.setNameToGroup()
	r.setGroupsWithStateChanges()
	r.setGroupToNames()
	r.setGroupToHighestDeployDepth()

	r.setNameToState()

	return nil
}

func (r *Resources) initPreParseConfigCurr() error {
	r.containerDir = viper.GetString("resourceContainerDirPath")

	err := r.setContainerSubdirPaths()
	if err != nil {
		return err
	}

	err = r.setNameToPackageJson()
	if err != nil {
		return err
	}

	r.setPackageJsonNameToName()

	r.setNameToDeps()

	err = r.setNameToIndexFilePath()
	if err != nil {
		return err
	}

	err = r.setNameToIndexFileContent()
	if err != nil {
		return err
	}

	return nil
}

func (r *Resources) initParseConfigCurr() error {
	r.setNameToConfigData()

	r.setNodeJsConfigScript()
	err := r.runNodeJsConfigScript()
	if err != nil {
		return err
	}

	return nil
}

func (r *Resources) initPostConfigCurr() {
	r.setNameToConfig()
	r.setNameToState()
}

func (r *Resources) initUp() error {
	r.upJsonPath = viper.GetString("upJsonPath")

	err := r.setUpJson()
	if err != nil {
		return err
	}

	r.setUpNameToDeps()
	r.setUpNameToConfig()
	r.setUpNameToOutput()

	return nil
}

type containerSubdirPaths []string

func (r *Resources) setContainerSubdirPaths() error {
	entries, err := os.ReadDir(r.containerDir)

	if err != nil {
		return fmt.Errorf("unable to read resource container dir %s", r.containerDir)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			r.containerSubdirPaths = append(r.containerSubdirPaths, filepath.Join(r.containerDir, entry.Name()))
		}
	}

	return nil
}

type nameToPackageJson map[string]*packageJson

type packageJson struct {
	Name            string            `json:"name"`
	Main            string            `json:"main"`
	Types           string            `json:"types"`
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies,omitempty"`
	DevDependencies map[string]string `json:"devDependencies,omitempty"`
}

func (r *Resources) setNameToPackageJson() error {
	r.nameToPackageJson = make(nameToPackageJson)

	for _, subdirPath := range r.containerSubdirPaths {
		resourceName := convertContainerSubdirPathToName(subdirPath)

		packageJsonPath := filepath.Join(subdirPath, "package.json")

		data, err := os.ReadFile(packageJsonPath)
		if err != nil {
			return fmt.Errorf("unable to read file %s\n%v", packageJsonPath, err)
		}

		var packageJson packageJson
		err = json.Unmarshal(data, &packageJson)
		if err != nil {
			return fmt.Errorf("unable to parse %s\n%v", packageJsonPath, err)
		}

		r.nameToPackageJson[resourceName] = &packageJson
	}

	return nil
}

func convertContainerSubdirPathToName(subdirPath string) string {
	subdirName := filepath.Base(subdirPath)
	snakeCaseResourceName := strings.ReplaceAll(subdirName, "-", "_")
	screamingSnakeCaseResourceName := strings.ToUpper(snakeCaseResourceName)
	return screamingSnakeCaseResourceName
}

type packageJsonNameToName map[string]string

func (r *Resources) setPackageJsonNameToName() {
	r.packageJsonNameToName = make(packageJsonNameToName)
	for resourceName, packageJson := range r.nameToPackageJson {
		r.packageJsonNameToName[packageJson.Name] = resourceName
	}
}

type nameToDeps map[string][]string

/*
A dependency is a resource the source resource depends on.
*/
func (r *Resources) setNameToDeps() {
	r.nameToDeps = make(nameToDeps)
	for resourceName, packageJson := range r.nameToPackageJson {
		var deps []string
		// Loop over source resource's package.json deps
		for dep := range packageJson.Dependencies {
			internalDep, ok := r.packageJsonNameToName[dep]
			// If package.json dep exists in map then it's an internal dep
			if ok {
				deps = append(deps, internalDep)
			}
		}
		r.nameToDeps[resourceName] = deps
	}
}

type nameToIndexFilePath map[string]string

func (r *Resources) setNameToIndexFilePath() error {
	r.nameToIndexFilePath = make(nameToIndexFilePath)

	indexFilePathPattern := regexp.MustCompile(`^_[^.]+\.[^.]+\.[^.]+\.index\.ts$`)

	for _, subdirPath := range r.containerSubdirPaths {
		subdirName := filepath.Base(subdirPath)
		snakeCaseResourceName := strings.ReplaceAll(subdirName, "-", "_")
		screamingSnakeCaseResourceName := strings.ToUpper(snakeCaseResourceName)

		srcPath := filepath.Join(subdirPath, "src")

		files, err := os.ReadDir(srcPath)
		if err != nil {
			return err
		}

		for _, file := range files {
			if !file.IsDir() && indexFilePathPattern.MatchString(file.Name()) {
				r.nameToIndexFilePath[screamingSnakeCaseResourceName] = filepath.Join(srcPath, file.Name())
				break
			}
		}
	}

	return nil
}

type nameToIndexFileContent map[string]string

func (r *Resources) setNameToIndexFileContent() error {
	r.nameToIndexFileContent = make(nameToIndexFileContent)
	for name, indexFilePath := range r.nameToIndexFilePath {
		data, err := os.ReadFile(indexFilePath)
		if err != nil {
			return fmt.Errorf("unable to read file %s\n%v", indexFilePath, err)
		}
		r.nameToIndexFileContent[name] = string(data)
	}
	return nil
}

type nameToConfigData map[string]*configData

type configData struct {
	variableName string
	functionName string
	exportString string
}

func (r *Resources) setNameToConfigData() {
	r.nameToConfigData = make(nameToConfigData)

	for name, indexFileContent := range r.nameToIndexFileContent {
		// Config setters are imported like this:
		// import { cloudflareKv } from "@gasoline-dev/resources"
		// They can be distinguished using a camelCase pattern.
		configSetterFunctionNameRegex := regexp.MustCompile(`import\s+\{[^}]*\b([a-z]+[A-Z][a-zA-Z]*)\b[^}]*\}\s+from\s+['"]@gasoline-dev/resources['"]`)
		// This can be limited to one match because there should only
		// be one config setter per resource index file.
		configSetterFunctionName := configSetterFunctionNameRegex.FindStringSubmatch(indexFileContent)[1]

		// Configs are exported like this:
		// export const coreBaseKv = cloudflareKv({
		//   name: "CORE_BASE_KV",
		// } as const)
		exportedConfigRegex := regexp.MustCompile(`(?m)export\s+const\s+\w+\s*=\s*\w+\([\s\S]*?\)\s*(?:as\s*const\s*)?;?`)

		// It can't be assumed that text that matches the exported config
		// pattern is an exported config. A user can export non-configs
		// using the same pattern above. So we need to collect possible
		// exported configs and evaluate them later.
		possibleExportedConfigs := exportedConfigRegex.FindAllString(indexFileContent, -1)

		// This regex matches the variable name of an exported
		// config. For example, it'd match "coreBaseKv" in:
		// export const coreBaseKv = cloudflareKv({
		//   name: "CORE_BASE_KV",
		// } as const)
		possibleExportedConfigVariableNameRegex := regexp.MustCompile(`export\s+const\s+(\w+)\s*=\s*\w+\(`)

		// This regex matches the function name of an exported
		// config. For example, it'd match "cloudflareKv" in:
		// export const coreBaseKv = cloudflareKv({
		//   name: "CORE_BASE_KV",
		// } as const)
		functionNameRegex := regexp.MustCompile(`\s*=\s*(\w+)\(`)

		for _, possibleExportedConfig := range possibleExportedConfigs {
			possibleExportedConfigFunctionName := functionNameRegex.FindStringSubmatch(possibleExportedConfig)[1]

			// If possible exported config function name is equal to the
			// config setter function name, then the possible exported
			// config function name and possible exported config are
			// confirmed to represent actual configs.
			if possibleExportedConfigFunctionName == configSetterFunctionName {
				r.nameToConfigData[name] = &configData{
					variableName: possibleExportedConfigVariableNameRegex.FindStringSubmatch(possibleExportedConfig)[1],
					functionName: possibleExportedConfigFunctionName,
					exportString: possibleExportedConfig,
				}
				break
			}
		}
	}
}

type nodeJsConfigScript = string

func (r *Resources) setNodeJsConfigScript() {
	var functionNames []string

	functionNameToTrue := make(map[string]bool)
	for _, configData := range r.nameToConfigData {
		functionNameToTrue[configData.functionName] = true
		functionNames = append(functionNames, configData.functionName)
	}

	r.nodeJsConfigScript = "import {\n"
	r.nodeJsConfigScript += strings.Join(functionNames, ",\n")
	r.nodeJsConfigScript += "\n} "
	r.nodeJsConfigScript += "from \"@gasoline-dev/resources\"\n"

	// Configs have to be written in bottom-up dependency order to
	// avoid Node.js "cannot access 'variable name' before
	// initialization" errors. For example, given a graph of A->B,
	// B's config has to be written before A's because A will
	// reference B's config.
	for group := range r.groupToDepthToNames {
		numOfDepths := len(r.groupToDepthToNames[group])
		for depth := numOfDepths; depth >= 0; depth-- {
			for _, name := range r.groupToDepthToNames[group][depth] {
				r.nodeJsConfigScript += strings.Replace(r.nameToConfigData[name].exportString, " as const", "", 1)
				r.nodeJsConfigScript += "\n"
			}
		}
	}

	r.nodeJsConfigScript += "const resourceNameToConfig = {}\n"

	for name, configData := range r.nameToConfigData {
		r.nodeJsConfigScript += fmt.Sprintf("resourceNameToConfig[\"%s\"] = %s\n", name, configData.variableName)
	}

	r.nodeJsConfigScript += "console.log(JSON.stringify(resourceNameToConfig))\n"
}

type runNodeJsConfigScriptResult map[string]interface{}

func (r *Resources) runNodeJsConfigScript() error {
	cmd := exec.Command("node", "--input-type=module")

	cmd.Stdin = bytes.NewReader([]byte(r.nodeJsConfigScript))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("unable to execute Node.js config script: %s\n%v", string(output), err)
	}

	strOutput := strings.TrimSpace(string(output))

	err = json.Unmarshal([]byte(strOutput), &r.runNodeJsConfigScriptResult)
	if err != nil {
		return fmt.Errorf("unable to marshall Node.js config script result\n%v", err)
	}

	return nil
}

type nameToConfig = map[string]interface{}

func (r *Resources) setNameToConfig() {
	r.nameToConfig = make(nameToConfig)
	for name, config := range r.runNodeJsConfigScriptResult {
		c := config.(map[string]interface{})
		resourceType := c["type"].(string)
		r.nameToConfig[name] = configs[resourceType](config.(map[string]interface{}))
	}
}

type upJson map[string]struct {
	Config       interface{} `json:"config"`
	Dependencies []string    `json:"dependencies"`
	Output       interface{} `json:"output"`
}

func (r *Resources) setUpJson() error {
	r.upJson = make(upJson)

	data, err := os.ReadFile(r.upJsonPath)
	if err != nil {
		return fmt.Errorf("unable to read up .json file %s\n%v", r.upJsonPath, err)
	}

	err = json.Unmarshal(data, &r.upJson)
	if err != nil {
		return fmt.Errorf("unable to marshall up .json file %s\n%v", r.upJsonPath, err)
	}

	return nil
}

type upNameToDeps map[string][]string

func (r *Resources) setUpNameToDeps() {
	r.upNameToDeps = make(upNameToDeps)
	for name, data := range r.upJson {
		dependencies := data.Dependencies
		if len(dependencies) > 0 {
			r.upNameToDeps[name] = dependencies
		} else {
			r.upNameToDeps[name] = make([]string, 0)
		}
	}
}

type upNameToConfig map[string]interface{}

func (r *Resources) setUpNameToConfig() upNameToConfig {
	r.upNameToConfig = make(upNameToConfig)
	for name, data := range r.upJson {
		config := data.Config.(map[string]interface{})
		resourceType := config["type"].(string)
		r.upNameToConfig[name] = configs[resourceType](config)
	}
	return r.upNameToConfig
}

type upNameToOutput map[string]interface{}

func (r *Resources) setUpNameToOutput() {
	r.upNameToOutput = make(upNameToOutput)
	for name, data := range r.upJson {
		output := data.Output.(map[string]interface{})
		r.upNameToOutput[name] = upOutputs["cloudflare-kv"](output)
	}
}

var upOutputs = map[string]func(output upOutput) interface{}{
	"cloudflare-kv": func(output upOutput) interface{} {
		return &CloudflareKvUpOutput{
			ID: output["id"].(string),
		}
	},
}

type upOutput map[string]interface{}

type CloudflareKvUpOutput struct {
	ID string `json:"id"`
}

/*
A group is an int assigned to resources that share
at least one common relative.
*/
func (r *Resources) setNameToGroup() {
	r.nameToGroup = make(nameToGroup)

	group := 0
	for _, sourceName := range r.namesWithInDegreesOfZero {
		if _, ok := r.nameToGroup[sourceName]; !ok {
			// Initialize source resource's group.
			r.nameToGroup[sourceName] = group

			// Set group for source resource's intermediates.
			for _, intermediate := range r.nameToIntermediates[sourceName] {
				if _, ok := r.nameToGroup[intermediate]; !ok {
					r.nameToGroup[intermediate] = group
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
			for _, possibleDistantRelativeName := range r.namesWithInDegreesOfZero {
				// Skip source resource from the main for loop.
				if possibleDistantRelativeName != sourceName {
					// Loop over possible distant relative's intermediates.
					for _, possibleDistantRelativeIntermediateName := range r.nameToIntermediates[possibleDistantRelativeName] {
						// Check if possible distant relative's intermediate
						// is also an intermediate of source resource.
						if helpers.IsStringInSlice(r.nameToIntermediates[sourceName], possibleDistantRelativeIntermediateName) {
							// If so, possible distant relative and source resource
							// are distant relatives and belong to the same group.
							r.nameToGroup[possibleDistantRelativeName] = group
						}
					}
				}
			}
			group++
		}
	}
}

type groupsWithStateChanges = []int

func (r *Resources) setGroupsWithStateChanges() {
	r.groupsWithStateChanges = make(groupsWithStateChanges, 0)
	seenGroups := make(map[int]struct{})
	for name, state := range r.nameToState {
		if state != stateType(UNCHANGED) {
			group, ok := r.nameToGroup[name]
			if ok {
				if _, alreadyAdded := seenGroups[group]; !alreadyAdded {
					r.groupsWithStateChanges = append(r.groupsWithStateChanges, group)
					seenGroups[group] = struct{}{}
				}
			}
		}
	}
}

type groupToNames map[int][]string

func (r *Resources) setGroupToNames() {
	r.groupToNames = make(groupToNames)
	for name, group := range r.nameToGroup {
		if _, ok := r.groupToNames[group]; !ok {
			r.groupToNames[group] = make([]string, 0)
		}
		r.groupToNames[group] = append(r.groupToNames[group], name)
	}
}

type groupToHighestDeployDepth map[int]int

func (r *Resources) setGroupToHighestDeployDepth() {
	r.groupToHighestDeployDepth = make(groupToHighestDeployDepth)
	for _, group := range r.groupsWithStateChanges {
		deployDepth := 0
		isFirstResourceToProcess := true
		for _, name := range r.groupToNames[group] {
			// UNCHANGED resources aren't deployed, so its depth
			// can't be the deploy depth.
			if r.nameToState[name] == stateType("UNCHANGED") {
				continue
			}

			// If resource is first to make it this far set deploy
			// depth so it can be used for comparison in future loops.
			if isFirstResourceToProcess {
				r.groupToHighestDeployDepth[group] = r.nameToDepth[name]
				deployDepth = r.nameToDepth[name]
				isFirstResourceToProcess = false
				continue
			}

			// Update deploy depth if resource's depth is greater than
			// the comparative deploy depth.
			if r.nameToDepth[name] > deployDepth {
				r.groupToHighestDeployDepth[group] = r.nameToDepth[name]
				deployDepth = r.nameToDepth[name]
			}
		}
	}
}

type nameToState map[string]stateType

type stateType string

const (
	CREATED   stateType = "CREATED"
	DELETED   stateType = "DELETED"
	UNCHANGED stateType = "UNCHANGED"
	UPDATED   stateType = "UPDATED"
)

func (r *Resources) setNameToState() {
	r.nameToState = make(nameToState)

	for name := range r.upNameToConfig {
		if _, ok := r.nameToConfig[name]; !ok {
			r.nameToState[name] = stateType(DELETED)
		}
	}

	for name := range r.nameToConfig {
		if _, ok := r.upNameToConfig[name]; !ok {
			r.nameToState[name] = stateType(CREATED)
		} else {
			if !reflect.DeepEqual(r.upNameToConfig[name], r.nameToConfig[name]) {
				r.nameToState[name] = stateType(UPDATED)
				continue
			}

			if !reflect.DeepEqual(r.upNameToDeps[name], r.nameToDeps[name]) {
				r.nameToState[name] = stateType(UPDATED)
				continue
			}

			r.nameToState[name] = stateType(UNCHANGED)
		}
	}
}

func (r *Resources) HasNamesToDeploy() bool {
	for name := range r.nameToState {
		if r.nameToState[name] != stateType(UNCHANGED) {
			return true
		}
	}
	return false
}

func (r *Resources) Deploy() error {
	r.logNamePreDeployStates()

	r.nameToDeployStateContainer = &nameToDeployStateContainer{
		m: make(map[string]deployState),
	}

	r.setNameToDeployStateOfPending()

	r.nameToDeployOutputContainer = &nameToDeployOutputContainer{
		m: make(map[string]interface{}),
	}

	err := r.deployGroups()
	if err != nil {
		return err
	}

	newUpjson := make(upJson)

	for resourceName, output := range r.nameToDeployOutputContainer.m {
		newUpjson[resourceName] = struct {
			Config       interface{} `json:"config"`
			Dependencies []string    `json:"dependencies"`
			Output       interface{} `json:"output"`
		}{
			Config:       r.nameToConfig[resourceName],
			Dependencies: r.nameToDeps[resourceName],
			Output:       output,
		}
	}

	jsonData, err := json.MarshalIndent(newUpjson, "", "  ")
	if err != nil {
		fmt.Println("error marshalling to JSON:", err)
		return nil
	}

	fileName := "gas.up.json"
	err = os.WriteFile(fileName, jsonData, 0644)
	if err != nil {
		fmt.Println("error writing to file:", err)
		return nil
	}

	return nil
}

type nameToGroup map[string]int

func (r *Resources) logNamePreDeployStates() {
	fmt.Println("# Pre-Deploy States:")
	for group, depthToNames := range r.groupToDepthToNames {
		for depth, names := range depthToNames {
			for _, name := range names {
				fmt.Printf(
					"Group %d -> Depth %d -> %s -> %s\n",
					group,
					depth,
					name,
					r.nameToState[name],
				)
			}
		}
	}
}

type nameToDeployStateContainer struct {
	m  map[string]deployState
	mu sync.Mutex
}

type deployState string

const (
	CANCELED           deployState = "CANCELED"
	CREATE_COMPLETE    deployState = "CREATE_COMPLETE"
	CREATE_FAILED      deployState = "CREATE_FAILED"
	CREATE_IN_PROGRESS deployState = "CREATE_IN_PROGRESS"
	DELETE_COMPLETE    deployState = "DELETE_COMPLETE"
	DELETE_FAILED      deployState = "DELETE_FAILED"
	DELETE_IN_PROGRESS deployState = "DELETE_IN_PROGRESS"
	PENDING            deployState = "PENDING"
	UPDATE_COMPLETE    deployState = "UPDATE_COMPLETE"
	UPDATE_FAILED      deployState = "UPDATE_FAILED"
	UPDATE_IN_PROGRESS deployState = "UPDATE_IN_PROGRESS"
)

func (r *Resources) logNameDeployState(name string, group int, depth int, timestamp int64) {
	date := time.Unix(0, timestamp*int64(time.Millisecond))
	hours := fmt.Sprintf("%02d", date.Hour())
	minutes := fmt.Sprintf("%02d", date.Minute())
	seconds := fmt.Sprintf("%02d", date.Second())
	formattedTime := fmt.Sprintf("%s:%s:%s", hours, minutes, seconds)

	fmt.Printf("[%s] Group %d -> Depth %d -> %s -> %s\n",
		formattedTime,
		group,
		depth,
		name,
		r.nameToDeployStateContainer.m[name],
	)
}

func (r *Resources) setNameToDeployStateOfComplete(name string) {
	r.nameToDeployStateContainer.mu.Lock()
	defer r.nameToDeployStateContainer.mu.Unlock()
	switch r.nameToDeployStateContainer.m[name] {
	case deployState(CREATE_IN_PROGRESS):
		r.nameToDeployStateContainer.m[name] = deployState(CREATE_COMPLETE)
	case deployState(DELETE_IN_PROGRESS):
		r.nameToDeployStateContainer.m[name] = deployState(DELETE_COMPLETE)
	case deployState(UPDATE_IN_PROGRESS):
		r.nameToDeployStateContainer.m[name] = deployState(UPDATE_COMPLETE)
	}
}

func (r *Resources) setNameToDeployStateOfFailed(name string) {
	r.nameToDeployStateContainer.mu.Lock()
	defer r.nameToDeployStateContainer.mu.Unlock()
	switch r.nameToDeployStateContainer.m[name] {
	case deployState(CREATE_IN_PROGRESS):
		r.nameToDeployStateContainer.m[name] = deployState(CREATE_FAILED)
	case deployState(DELETE_IN_PROGRESS):
		r.nameToDeployStateContainer.m[name] = deployState(DELETE_FAILED)
	case deployState(UPDATE_IN_PROGRESS):
		r.nameToDeployStateContainer.m[name] = deployState(UPDATE_FAILED)
	}
}

func (r *Resources) setNameToDeployStateOfInProgress(name string) {
	r.nameToDeployStateContainer.mu.Lock()
	defer r.nameToDeployStateContainer.mu.Unlock()
	switch r.nameToState[name] {
	case stateType(CREATED):
		r.nameToDeployStateContainer.m[name] = deployState(CREATE_IN_PROGRESS)
	case stateType(DELETED):
		r.nameToDeployStateContainer.m[name] = deployState(DELETE_IN_PROGRESS)
	case stateType(UPDATED):
		r.nameToDeployStateContainer.m[name] = deployState(UPDATE_IN_PROGRESS)
	}
}

func (r *Resources) setNameToDeployStateOfPending() {
	r.nameToDeployStateContainer.mu.Lock()
	defer r.nameToDeployStateContainer.mu.Unlock()
	for name, state := range r.nameToState {
		if state != stateType(UNCHANGED) {
			r.nameToDeployStateContainer.m[name] = deployState(PENDING)
		}
	}
}

func (r *Resources) setNameToDeployStatePendingOfCanceled() int {
	r.nameToDeployStateContainer.mu.Lock()
	defer r.nameToDeployStateContainer.mu.Unlock()
	result := 0
	for name, state := range r.nameToDeployStateContainer.m {
		if state == deployState(PENDING) {
			r.nameToDeployStateContainer.m[name] = deployState(CANCELED)
		}
	}
	return result
}

type deployGroupOkChanType chan bool

func (r *Resources) deployGroups() error {
	numOfGroupsToDeploy := len(r.groupsWithStateChanges)

	deployGroupOkChan := make(deployGroupOkChanType)

	for _, group := range r.groupsWithStateChanges {
		go r.deployGroup(group, deployGroupOkChan)
	}

	numOfGroupsDeployedOk := 0
	numOfGroupsDeployedErr := 0

	for groupDeployedOk := range deployGroupOkChan {
		if groupDeployedOk {
			numOfGroupsDeployedOk++
		} else {
			numOfGroupsDeployedErr++
		}

		numOfGroupsFinishedDeploying := numOfGroupsDeployedOk + numOfGroupsDeployedErr

		if numOfGroupsFinishedDeploying == numOfGroupsToDeploy {
			if numOfGroupsDeployedErr > 0 {
				return fmt.Errorf("deployment failed")
			}
			break
		}
	}

	return nil
}

func (r *Resources) deployGroup(group int, deployGroupOkChan deployGroupOkChanType) {
	deployNameOkChan := make(deployNameOkChanType)

	highestGroupDeployDepth := r.groupToHighestDeployDepth[group]

	initialGroupResourceNamesToDeploy := r.setInitGroupNamesToDeploy(
		highestGroupDeployDepth,
		group,
	)

	for _, name := range initialGroupResourceNamesToDeploy {
		depth := r.nameToDepth[name]
		go r.deployName(name, deployNameOkChan, group, depth)
	}

	numOfNamesInGroupToDeploy := r.setNumInGroupToDeploy(
		group,
	)

	numOfNamesDeployedOk := 0
	numOfNamesDeployedErr := 0
	numOfNamesDeployedCanceled := 0

	for nameDeployedOk := range deployNameOkChan {
		if nameDeployedOk {
			numOfNamesDeployedOk++
		} else {
			numOfNamesDeployedErr++
			// Cancel PENDING resources.
			// Check for 0 because resources should only
			// be canceled one time.
			if numOfNamesDeployedCanceled == 0 {
				numOfNamesDeployedCanceled = r.setNameToDeployStatePendingOfCanceled()
			}
		}

		numOfNamesInFinalDeployState := numOfNamesDeployedOk +
			numOfNamesDeployedErr +
			numOfNamesDeployedCanceled

		if numOfNamesInFinalDeployState == int(numOfNamesInGroupToDeploy) {
			if numOfNamesDeployedErr == 0 {
				deployGroupOkChan <- true
			} else {
				deployGroupOkChan <- false
			}
			return
		} else {
			for _, name := range r.groupToNames[group] {
				if r.nameToDeployStateContainer.m[name] == deployState("PENDING") {
					shouldDeployResource := true

					// Is resource dependent on another deploying resource?
					for _, dep := range r.nameToDeps[name] {
						activeStates := map[deployState]bool{
							deployState(CREATE_IN_PROGRESS): true,
							deployState(DELETE_IN_PROGRESS): true,
							deployState(PENDING):            true,
							deployState(UPDATE_IN_PROGRESS): true,
						}

						depDeployState := r.nameToDeployStateContainer.m[dep]

						if activeStates[depDeployState] {
							shouldDeployResource = false
							break
						}
					}

					if shouldDeployResource {
						depth := r.nameToDepth[name]
						go r.deployName(name, deployNameOkChan, group, depth)
					}
				}
			}
		}
	}
}

type initGroupNamesToDeploy []string

/*
Deployments can't only start at the highest depth
containing a resource to deploy (i.e. a resource
with a deploy state of PENDING).

For example, given a graph of:
a -> b
b -> c
c -> d
a -> e

d has a depth of 3 and e has a depth of 1.

If just d and e need to be deployed, the deployment can't start
at depth 3 only. e would be blocked until d finished because
d has a higher depth than e. That's not optimal. They should
be started at the same time and deployed concurrently.
*/
func (r *Resources) setInitGroupNamesToDeploy(
	highestDepthContainingAResourceToDeploy int,
	group int,
) initGroupNamesToDeploy {
	var result initGroupNamesToDeploy

	// Add every resource at highest deploy depth containing
	// a resource to deploy.
	result = append(result, r.groupToDepthToNames[group][highestDepthContainingAResourceToDeploy]...)

	// Check all other depths, except 0, for resources that can
	// start deploying on deployment initiation (0 is skipped
	// because a resource at that depth can only be deployed
	// first if it's being deployed in isolation).
	depthToCheck := highestDepthContainingAResourceToDeploy - 1
	for depthToCheck > 0 {
		for _, resourceNameAtDepthToCheck := range r.groupToDepthToNames[group][depthToCheck] {
			for _, dependencyName := range r.nameToDeps[resourceNameAtDepthToCheck] {
				// If resource at depth to check is PENDING and is not
				// dependent on any resource in the ongoing result, then
				// append it to the result.
				if r.nameToDeployStateContainer.m[resourceNameAtDepthToCheck] == deployState(PENDING) && !helpers.IsStringInSlice(result, dependencyName) {
					result = append(result, resourceNameAtDepthToCheck)
				}
			}
		}
		depthToCheck--
	}

	return result
}

type numInGroupToDeploy int

func (r *Resources) setNumInGroupToDeploy(group int) numInGroupToDeploy {
	result := numInGroupToDeploy(0)
	for _, resourceName := range r.groupToNames[group] {
		if r.nameToState[resourceName] != stateType(UNCHANGED) {
			result++
		}
	}
	return result
}

type nameToDeployOutputContainer struct {
	m  map[string]interface{}
	mu sync.Mutex
}

func (r *Resources) setNameToDeployOutput(name string, key processorKeyType, output interface{}) {
	r.nameToDeployOutputContainer.mu.Lock()
	defer r.nameToDeployOutputContainer.mu.Unlock()
	r.nameToDeployOutputContainer.m[name] = resourceDeployOutputs[key](output)
}

type deployNameOkChanType chan bool

func (r *Resources) deployName(name string, deployNameOkChan deployNameOkChanType, group int, depth int) {
	r.setNameToDeployStateOfInProgress(name)

	timestamp := time.Now().UnixMilli()

	r.logNameDeployState(name, group, depth, timestamp)

	processorOkChan := make(processorOkChanType)

	resourceType := reflect.ValueOf(r.nameToConfig[name]).Elem().FieldByName("Type").String()

	processorKey := processorKeyType(resourceType + ":" + string(r.nameToState[name]))

	go processorsNew[processorKey](r.nameToConfig[name], processorOkChan, r.nameToDeployOutputContainer, r.upNameToOutput[name])

	if <-processorOkChan {
		r.setNameToDeployStateOfComplete(name)

		timestamp = time.Now().UnixMilli()

		r.logNameDeployState(name, group, depth, timestamp)

		deployNameOkChan <- true

		return
	}

	r.setNameToDeployStateOfFailed(name)

	timestamp = time.Now().UnixMilli()

	r.logNameDeployState(name, group, depth, timestamp)

	deployNameOkChan <- false
}

type processorsType map[processorKeyType]func(
	config interface{},
	processOkChan processorOkChanType,
	deployOutput *nameToDeployOutputContainer,
	upOutput interface{},
)

type processorOkChanType = chan bool

type processorKeyType string

const (
	CLOUDFLARE_KV_CREATED processorKeyType = "cloudflare-kv:CREATED"
	CLOUDFLARE_KV_DELETED processorKeyType = "cloudflare-kv:DELETED"
)

// TODO: Think about changing into a register pattern like monkey
var processorsNew processorsType = processorsType{
	CLOUDFLARE_KV_CREATED: func(
		config interface{},
		processOkChan processorOkChanType,
		deployOutput *nameToDeployOutputContainer,
		upOutput interface{},
	) {
		c := config.(*CloudflareKVConfig)

		api, err := cloudflare.NewWithAPIToken(os.Getenv("CLOUDFLARE_API_TOKEN"))
		if err != nil {
			fmt.Println("Error:", err)
			processOkChan <- false
			return
		}

		title := viper.GetString("project") + "-" + helpers.CapitalSnakeCaseToTrainCase(c.Name)

		req := cloudflare.CreateWorkersKVNamespaceParams{Title: title}

		res, err := api.CreateWorkersKVNamespace(context.Background(), cloudflare.AccountIdentifier(os.Getenv("CLOUDFLARE_ACCOUNT_ID")), req)

		if err != nil {
			fmt.Println("Error:", err)
			processOkChan <- false
			return
		}

		// TODO: THIS NEEDS TO BE FIXED
		//deployOutput.setNameToDeployOutput()
		// deployOutput.set(c.Name, CLOUDFLARE_KV_CREATED, res)

		fmt.Println(res)

		processOkChan <- true
	},
	CLOUDFLARE_KV_DELETED: func(
		config interface{},
		processOkChan processorOkChanType,
		deployOutput *nameToDeployOutputContainer,
		upOutput interface{},
	) {
		uo := upOutput.(*CloudflareKvUpOutput)

		api, err := cloudflare.NewWithAPIToken(os.Getenv("CLOUDFLARE_API_TOKEN"))
		if err != nil {
			fmt.Println("Error:", err)
			processOkChan <- false
			return
		}

		res, err := api.DeleteWorkersKVNamespace(context.Background(), cloudflare.AccountIdentifier(os.Getenv("CLOUDFLARE_ACCOUNT_ID")), uo.ID)

		fmt.Println(res)

		if err != nil {
			fmt.Println("Error:", err)
			processOkChan <- false
			return
		}

		processOkChan <- true
	},
}

var configs = map[string]func(config config) interface{}{
	"cloudflare-kv": func(config config) interface{} {
		return &CloudflareKVConfig{
			ConfigCommon: ConfigCommon{
				Type: config["type"].(string),
				Name: config["name"].(string),
			},
		}
	},
}

type config map[string]interface{}

type ConfigCommon struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type CloudflareKVConfig struct {
	ConfigCommon
}

type CloudflareWorkerConfig struct {
	ConfigCommon
	KV []struct {
		Binding string `json:"binding"`
	} `json:"kv"`
}

var resourceDeployOutputs = map[processorKeyType]func(output interface{}) interface{}{
	CLOUDFLARE_KV_CREATED: func(res interface{}) interface{} {
		r := res.(cloudflare.WorkersKVNamespaceResponse)

		return &CloudflareKVOutput{
			ID: r.Result.ID,
		}
	},
}

type CloudflareKVOutput struct {
	ID string `json:"id"`
}
