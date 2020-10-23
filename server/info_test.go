package server

import (
	"encoding/json"
	"fmt"
	"testing"
)

func Test_info(t *testing.T) {
	jsonstr := `{"links":[{"href":"https://cobaltkiss.blue/nodeinfo/2.0.json","rel":"http://nodeinfo.diaspora.software/ns/schema/2.0"},{"href":"https://cobaltkiss.blue/nodeinfo/2.1.json","rel":"http://nodeinfo.diaspora.software/ns/schema/2.1"}]}`
	var a map[string]interface{}
	err := json.Unmarshal([]byte(jsonstr), &a)
	fmt.Println(err, a)
}
