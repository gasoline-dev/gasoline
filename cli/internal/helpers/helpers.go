package helpers

import (
	"encoding/json"
	"fmt"
	"os"
)

func IsFilePresent(filePath string) bool {
	_, err := os.Stat(filePath)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

func IsInSlice(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

type MergeMap map[string]interface{}

func MergeMaps(map1, map2 MergeMap) MergeMap {
	result := make(MergeMap)
	for key, value := range map1 {
		result[key] = value
	}
	for key, value := range map2 {
		result[key] = value
	}
	return result
}

func PrettyPrint(data interface{}) error {
	jsonData, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return err
	}
	fmt.Println(string(jsonData))
	return nil
}

func ReadFile(filePath string) ([]byte, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("unable to read %s\n%v", filePath, err)
	}
	return data, nil
}

func UnmarshallFile(filePath string, pointer any) error {
	data, err := ReadFile(filePath)
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, pointer)
	if err != nil {
		return fmt.Errorf("unable to parse %s\n%v", filePath, err)
	}

	return nil
}

func WriteFile(filePath, data string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("unable to open %s\n%v", filePath, err)
	}
	defer file.Close()

	_, err = file.WriteString(data)
	if err != nil {
		return fmt.Errorf("unable to write %s\n%v", filePath, err)
	}

	return nil
}
