package helpers

import (
	"encoding/json"
	"fmt"
)

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
