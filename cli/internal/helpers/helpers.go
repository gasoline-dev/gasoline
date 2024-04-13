package helpers

import (
	"encoding/json"
	"fmt"
)

/*
IsInSlice checks if an item is in a slice.
*/
func IsInSlice(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

/*
MergeMaps merges two maps.

map1 will overwrite keys in map2.
*/
func MergeMaps(map1, map2 map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for key, value := range map1 {
		result[key] = value
	}
	for key, value := range map2 {
		result[key] = value
	}
	return result
}

/*
PrettyPrint prints data in a pretty format.
*/
func PrettyPrint(data interface{}) error {
	jsonData, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return err
	}
	fmt.Println(string(jsonData))
	return nil
}
