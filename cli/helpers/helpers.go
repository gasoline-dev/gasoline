package helpers

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

/*
CORE_BASE_API -> Core-Base-Api
*/
func CapitalSnakeCaseToTrainCase(s string) string {
	caser := cases.Title(language.English)
	words := strings.Split(s, "_")
	for i, word := range words {
		words[i] = caser.String(strings.ToLower(word))
	}
	return strings.Join(words, "-")
}

// IncludesString checks if a string slice contains a given value.
func IncludesString(slice []string, value string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

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

type mergeMap map[string]any

func MergeInterfaceMaps(map1, map2 map[string]interface{}) map[string]interface{} {
	result := make(mergeMap)
	for key, value := range map1 {
		result[key] = value
	}
	for key, value := range map2 {
		result[key] = value
	}
	return result
}

func MergeStringSliceMaps(map1, map2 map[string][]string) map[string][]string {
	result := make(map[string][]string)
	for key, value := range map1 {
		result[key] = value
	}
	for key, value := range map2 {
		result[key] = value
	}
	return result
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
