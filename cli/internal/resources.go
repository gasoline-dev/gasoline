package resources

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

//go:embed embed/get-index-build-file-exports.js
var getIndexBuildFileExportsEmbed embed.FS

/*
GetContainerSubDirPaths returns a list of subdirectory paths in the
resource container directory. For example, ["gas/core-base-api"].
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
GetIndexFilePaths returns a list of index file paths in the resource
container subdirectories. For example,
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

func GetIndexBuildFileExports(indexBuildFilePaths []string) error {
	embedPath := "embed/get-index-build-file-exports.js"

	for _, filePath := range indexBuildFilePaths {
		fmt.Println(filePath)
		content, err := getIndexBuildFileExportsEmbed.ReadFile(embedPath)
		if err != nil {
			return fmt.Errorf("unable to read embed %s", embedPath)
		}

		nodeCmd := exec.Command("node", "--input-type=module")
		nodeCmd.Stdin = bytes.NewReader(content)
		output, err := nodeCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("unable to execute embed %s", embedPath)
		}

		strOutput := string(output)

		jsError := "Error: unable to get exports\n"

		if strings.Contains(strOutput, jsError) {
			strOutput = strings.Replace(strOutput, jsError, "", 1)

			return fmt.Errorf("unable to get exports in file %s\n%s", filePath, strOutput)
		}

		fmt.Println(strOutput)
	}

	return nil
}

/*
GetIndexBuildFilePaths returns a list of index build file paths in the resource container subdirectories. For example,
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

/*
ValidateContainerSubDirContents checks if the resource container subdirectories
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
