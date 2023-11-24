package misc

import (
	"encoding/json"

	yaml "gopkg.in/yaml.v2"
)

// YamlToJSON converts yaml to json string
func YamlToJSON(y string) (j string, err error) {
	var body interface{}
	err = yaml.Unmarshal([]byte(y), &body)
	if err != nil {
		return
	}

	body = convert(body)
	b, err := json.Marshal(body)
	if err != nil {
		return
	}
	j = string(b)
	return
}

func convert(i interface{}) interface{} {
	switch x := i.(type) {
	case map[interface{}]interface{}:
		m2 := map[string]interface{}{}
		for k, v := range x {
			m2[k.(string)] = convert(v)
		}
		return m2
	case []interface{}:
		for i, v := range x {
			x[i] = convert(v)
		}
	}
	return i
}
