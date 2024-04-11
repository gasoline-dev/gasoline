package resources

import "os"

func GetContainerSubDirs(containerDir string) ([]string, error) {
	var dirs []string

	entries, err := os.ReadDir(containerDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}

	return dirs, nil
}
