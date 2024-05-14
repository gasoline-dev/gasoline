package validators

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

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
			return fmt.Errorf("unable to walk path %q\n%v", subdirPath, err)
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
