package evmpunk

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
)

// ParseABIEvents extracts event definitions from an ABI file
func ParseABIEvents(abiPath string) ([]EventInfo, error) {
	data, err := os.ReadFile(abiPath)
	if err != nil {
		return nil, err
	}

	var abiJSON interface{}
	json.Unmarshal(data, &abiJSON)

	var abiArray []interface{}
	switch v := abiJSON.(type) {
	case []interface{}:
		abiArray = v
	case map[string]interface{}:
		if abi, ok := v["abi"].([]interface{}); ok {
			abiArray = abi
		}
	}

	var events []EventInfo
	for _, item := range abiArray {
		itemMap, ok := item.(map[string]interface{})
		if !ok || itemMap["type"] != "event" {
			continue
		}

		eventName := itemMap["name"].(string)
		inputs := itemMap["inputs"].([]interface{})

		var fields []EventField
		var sigTypes []string

		for _, input := range inputs {
			inputMap := input.(map[string]interface{})
			field := EventField{
				Name:    inputMap["name"].(string),
				Type:    inputMap["type"].(string),
				Indexed: inputMap["indexed"].(bool),
			}
			fields = append(fields, field)
			sigTypes = append(sigTypes, field.Type)
		}

		signature := fmt.Sprintf("%s(%s)", eventName, strings.Join(sigTypes, ","))
		hash := crypto.Keccak256Hash([]byte(signature))

		events = append(events, EventInfo{
			Name:      eventName,
			Fields:    fields,
			Signature: hash.Hex(),
		})
	}

	return events, nil
}
