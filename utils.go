package coremongo

import (
	"bytes"
	"encoding/json"
	"time"
)

func convertDates(input map[string]interface{}) map[string]interface{} {

	for key, value := range input {
		if value == "CURRENT_TIMESTAMP" {
			value = time.Now()
		}
		switch v := value.(type) {
		case time.Time:
			input[key] = value

		case map[string]interface{}:
			// Ricorsione: esplora i livelli interni del documento
			input[key] = convertDates(v)
		}
	}
	return input
}

func PrettyPrintJson(jsonStr []byte) (string, error) {
	var prettyJSON bytes.Buffer
	err := json.Indent(&prettyJSON, jsonStr, "", "  ")
	if err != nil {
		return "", err
	}
	return prettyJSON.String(), nil
}
