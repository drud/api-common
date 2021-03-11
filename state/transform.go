package state

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// TransformTag is a custom struct tag for transforming field paths
const TransformTag = "transform"

func genericMap(obj interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	var generic map[string]interface{}
	err = json.Unmarshal(data, &generic)
	if err != nil {
		return nil, err
	}
	return generic, nil
}

func Transform(in interface{}, out interface{}) error {
	generic, err := genericMap(in)
	if err != nil {
		return err
	}
	ret := make(map[string]interface{})
	t := reflect.TypeOf(out)
	// Iterate over all available fields and read the tag value
	for i := 0; i < t.Elem().NumField(); i++ {
		// Get the field, returns https://golang.org/pkg/reflect/#StructField
		field := t.Elem().Field(i)

		// Get the field tag value
		tag := field.Tag.Get(TransformTag)
		tagElements := strings.Split(tag, ",")
		name := tagElements[0]
		omitempty := false
		for _, option := range tagElements[1:] {
			if option == "omitempty" {
				omitempty = true
			}
		}
		_ = omitempty
		json := field.Tag.Get("json")
		current := generic
		if name != "" {
			ok := true
			path := strings.Split(name, ".")
			if path[0] == "" && len(path) > 1 {
				// Allow them to start a path with a `.` out of preference
				path = path[1:]
			}
			for i, key := range path {
				if i == len(path)-1 {
					ret[json] = current[key]
				} else {
					if current, ok = current[key].(map[string]interface{}); !ok {
						return fmt.Errorf("tag error received element %T expected map", current[key])
					}
				}
			}
		}
	}
	data, err := json.Marshal(ret)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, out)
	if err != nil {
		return err
	}

	return nil
}
