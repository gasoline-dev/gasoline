package resources

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
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
	nameToInternalDeps          nameToInternalDeps
	nameToIndexFilePath         nameToIndexFilePath
	nameToIndexFileContent      nameToIndexFileContent
	nameToConfigData            nameToConfigData
	groupToDepthToNames         graph.GroupToDepthToNodes
	nodeJsConfigScript          nodeJsConfigScript
	runNodeJsConfigScriptResult runNodeJsConfigScriptResult
	nameToConfig                nameToConfig
	upJsonPath                  string
	upJson                      upJson
	upNameToDeps                upNameToDeps
	upNameToConfig              upNameToConfig
	upNameToOutput              upNameToOutput
	nameToState                 nameToState
	stateToNames                stateToNames
	nameToDeployStateContainer  *nameToDeployStateContainer
}

func New() (*Resources, error) {
	r := &Resources{}
	err := r.init()
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Resources) init() error {
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

	r.setNameToInternalDeps()

	g := graph.New(graph.NodeToDeps(r.nameToInternalDeps))

	err = r.setNameToIndexFilePath()
	if err != nil {
		return err
	}

	err = r.setNameToIndexFileContent()
	if err != nil {
		return err
	}

	r.setNameToConfigData()

	r.groupToDepthToNames = g.GroupToDepthToNodes

	r.setNodeJsConfigScript()

	err = r.runNodeJsConfigScript()
	if err != nil {
		return err
	}

	r.setNameToConfig()

	r.upJsonPath = viper.GetString("upJsonPath")

	err = r.setUpJson()
	if err != nil {
		return err
	}

	r.setUpNameToDeps()

	r.setUpNameToConfig()

	r.setUpNameToOutput()

	r.setNameToState()

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

type nameToInternalDeps map[string][]string

func (r *Resources) setNameToInternalDeps() {
	r.nameToInternalDeps = make(nameToInternalDeps)
	for resourceName, packageJson := range r.nameToPackageJson {
		var internalDeps []string
		// Loop over source resource's package.json deps
		for dep := range packageJson.Dependencies {
			internalDep, ok := r.packageJsonNameToName[dep]
			// If package.json dep exists in map then it's an internal dep
			if ok {
				internalDeps = append(internalDeps, internalDep)
			}
		}
		r.nameToInternalDeps[resourceName] = internalDeps
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

func (r *Resources) HasNamesToDeploy() bool {
	for name := range r.nameToState {
		if r.nameToState[name] != stateType(UNCHANGED) {
			return true
		}
	}
	return false
}

func (r *Resources) Deploy() {
	r.nameToDeployStateContainer = &nameToDeployStateContainer{
		m: make(map[string]deployState),
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

			if !reflect.DeepEqual(r.upNameToDeps[name], r.nameToInternalDeps[name]) {
				r.nameToState[name] = stateType(UPDATED)
				continue
			}

			r.nameToState[name] = stateType(UNCHANGED)
		}
	}
}

type stateToNames = map[stateType][]string

func (r *Resources) setStateToNames() {
	r.stateToNames = make(stateToNames)
	for name, state := range r.nameToState {
		if _, ok := r.stateToNames[state]; !ok {
			r.stateToNames[state] = make([]string, 0)
		}
		r.stateToNames[state] = append(r.stateToNames[state], name)
	}
}

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

// TODO: change to logNameDeployState
func (r *Resources) logDeployState(group int, depth int, name string, timestamp int64) {
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

func (r *Resources) setNameToDeployStateAsComplete(name string) {
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

func (r *Resources) setNameToDeployStateAsFailed(name string) {
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

func (r *Resources) setNameToDeployStateAsInProgress(name string) {
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

func (r *Resources) setNameToDeployStateAsPending(name string) {
	r.nameToDeployStateContainer.mu.Lock()
	defer r.nameToDeployStateContainer.mu.Unlock()
	for name, state := range r.nameToState {
		if state != stateType(UNCHANGED) {
			r.nameToDeployStateContainer.m[name] = deployState(PENDING)
		}
	}
}

func (r *Resources) setNameToDeployStatePendingAsCanceled() int {
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

type IndexBuildFilePaths = []string

/*
["gas/core-base-api/build/core-base-api._index.js"]
*/
func GetIndexBuildFilePaths(containerSubdirPaths ContainerSubdirPaths) (IndexBuildFilePaths, error) {
	var result IndexBuildFilePaths

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
func GetIndexBuildFileConfigs(containerSubdirPaths ContainerSubdirPaths, indexFilePaths IndexFilePaths, indexBuildFilePaths IndexBuildFilePaths) (IndexBuildFileConfigs, error) {
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

type NameToConfig map[string]interface{}

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

type NameToDependencies map[string][]string

/*
TODO
*/
func SetNameToDependencies(indexBuildFileConfigs IndexBuildFileConfigs, dependencyNames DependencyNames) NameToDependencies {
	result := make(NameToDependencies)
	for index, config := range indexBuildFileConfigs {
		name := config["name"].(string)
		if dependencyNames[index] != nil {
			result[name] = dependencyNames[index]
		} else {
			result[name] = make([]string, 0)
		}
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

type NamesWithInDegreesOf []string

/*
TODO
*/
func SetNamesWithInDegreesOf(nameToInDegrees NameToInDegrees, degrees int) NamesWithInDegreesOf {
	var result NamesWithInDegreesOf
	for name, inDegree := range nameToInDegrees {
		if inDegree == degrees {
			result = append(result, name)
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
func SetNameToGroup(namesWithInDegreesOfZero NamesWithInDegreesOf, nameToIntermediateNames NameToIntermediateNames) NameToGroup {
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
func SetDepthToName(nameToDependencies NameToDependencies, namesWithInDegreesOfZero NamesWithInDegreesOf) DepthToName {
	result := make(DepthToName)

	numOfNamesToProcess := len(nameToDependencies)

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

type UpJson map[string]struct {
	Config       interface{} `json:"config"`
	Dependencies []string    `json:"dependencies"`
	Output       interface{} `json:"output"`
}

func GetUpJson(filePath string) (UpJson, error) {
	var result UpJson

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

type UpNameToDependencies map[string][]string

func SetUpNameToDependencies(upJson UpJson) UpNameToDependencies {
	result := make(UpNameToDependencies)
	for name, data := range upJson {
		dependencies := data.Dependencies
		if len(dependencies) > 0 {
			result[name] = dependencies
		} else {
			result[name] = make([]string, 0)
		}
	}
	return result
}

type UpNameToConfig map[string]interface{}

func SetUpNameToConfig(upJson UpJson) UpNameToConfig {
	result := make(UpNameToConfig)
	for name, data := range upJson {
		config := data.Config.(map[string]interface{})
		resourceType := config["type"].(string)
		result[name] = configs[resourceType](config)
	}
	return result
}

type UpNameToOutput map[string]interface{}

func SetUpNameToOutput(upJson UpJson) UpNameToOutput {
	result := make(UpNameToOutput)
	for name, data := range upJson {
		output := data.Output.(map[string]interface{})
		result[name] = upOutputs["cloudflare-kv"](output)
	}
	return result
}

type NameToState map[string]State

type State string

func SetNameToState(upNameToConfig UpNameToConfig, nameToConfig NameToConfig, upNameToDependencies UpNameToDependencies, nameToDependencies NameToDependencies) NameToState {
	result := make(NameToState)

	for name := range upNameToConfig {
		if _, ok := nameToConfig[name]; !ok {
			result[name] = State(DELETED)
		}
	}

	for name := range nameToConfig {
		if _, ok := upNameToConfig[name]; !ok {
			result[name] = State(CREATED)
		} else {
			if !reflect.DeepEqual(upNameToConfig[name], nameToConfig[name]) {
				result[name] = State(UPDATED)
				continue
			}

			if !reflect.DeepEqual(upNameToDependencies[name], nameToDependencies[name]) {
				result[name] = State(UPDATED)
				continue
			}

			result[name] = State(UNCHANGED)
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

type NameToDeployStateContainer struct {
	M  map[string]DeployState
	mu sync.Mutex
}

type DeployState string

func (c *NameToDeployStateContainer) Log(group int, depth int, name string, timestamp int64) {
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
		c.M[name],
	)
}

func (c *NameToDeployStateContainer) SetComplete(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch c.M[name] {
	case DeployState(CREATE_IN_PROGRESS):
		c.M[name] = DeployState(CREATE_COMPLETE)
	case DeployState(DELETE_IN_PROGRESS):
		c.M[name] = DeployState(DELETE_COMPLETE)
	case DeployState(UPDATE_IN_PROGRESS):
		c.M[name] = DeployState(UPDATE_COMPLETE)
	}
}

func (c *NameToDeployStateContainer) SetFailed(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch c.M[name] {
	case DeployState(CREATE_IN_PROGRESS):
		c.M[name] = DeployState(CREATE_FAILED)
	case DeployState(DELETE_IN_PROGRESS):
		c.M[name] = DeployState(DELETE_FAILED)
	case DeployState(UPDATE_IN_PROGRESS):
		c.M[name] = DeployState(UPDATE_FAILED)
	}
}

func (c *NameToDeployStateContainer) SetInProgress(name string, resourceNameToState NameToState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch resourceNameToState[name] {
	case State(CREATED):
		c.M[name] = DeployState(CREATE_IN_PROGRESS)
	case State(DELETED):
		c.M[name] = DeployState(DELETE_IN_PROGRESS)
	case State(UPDATED):
		c.M[name] = DeployState(UPDATE_IN_PROGRESS)
	}
}

func (c *NameToDeployStateContainer) SetPending(nameToState NameToState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for name, state := range nameToState {
		if state != State(UNCHANGED) {
			c.M[name] = DeployState(PENDING)
		}
	}
}

func (c *NameToDeployStateContainer) SetPendingToCanceled() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := 0
	for name, state := range c.M {
		if state == DeployState(PENDING) {
			c.M[name] = DeployState(CANCELED)
		}
	}
	return result
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

type processors map[ProcessorKey]func(
	config interface{},
	processOkChan ProcessorOkChan,
	deployOutput *NameToDeployOutputContainer,
	upOutput interface{},
)

type ProcessorOkChan = chan bool

type ProcessorKey string

const (
	CLOUDFLARE_KV_CREATED ProcessorKey = "cloudflare-kv:CREATED"
	CLOUDFLARE_KV_DELETED ProcessorKey = "cloudflare-kv:DELETED"
)

var Processors processors = processors{
	CLOUDFLARE_KV_CREATED: func(
		config interface{},
		processOkChan ProcessorOkChan,
		deployOutput *NameToDeployOutputContainer,
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

		deployOutput.set(c.Name, CLOUDFLARE_KV_CREATED, res)

		fmt.Println(res)

		processOkChan <- true
	},
	CLOUDFLARE_KV_DELETED: func(
		config interface{},
		processOkChan ProcessorOkChan,
		deployOutput *NameToDeployOutputContainer,
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

type NameToDeployOutputContainer struct {
	M  map[string]interface{}
	mu sync.Mutex
}

func (c *NameToDeployOutputContainer) set(name string, key ProcessorKey, output interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.M[name] = resourceDeployOutputs[key](output)
}

var resourceDeployOutputs = map[ProcessorKey]func(output interface{}) interface{}{
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
