package mongo

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestSwitchBachecaFunctions(t *testing.T) {
	sql := "SELECT * FROM users WHERE users.name = 'John' AND users.age < 30 AND users.age > 50"
	mongoQuery, err := ParseSQL(sql)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	jsonQuery, _ := json.MarshalIndent(mongoQuery, "", "  ")
	fmt.Println(string(jsonQuery))

}
