package helpers

import (
	"encoding/json"
	"fmt"
)

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
