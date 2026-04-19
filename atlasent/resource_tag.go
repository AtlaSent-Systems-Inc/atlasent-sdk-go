package atlasent

import (
	"fmt"
	"reflect"
	"strings"
)

// ResourceFrom derives a Resource from a tagged struct via reflection.
//
//	type Invoice struct {
//	    ID       string `atlasent:"id"`
//	    Customer string `atlasent:"attr,name=customer_id"`
//	    Amount   int    `atlasent:"attr"`
//	}
//
// Tag grammar:
//
//	`atlasent:"id"`            → Resource.ID comes from this field
//	`atlasent:"type"`          → Resource.Type comes from this field
//	`atlasent:"attr"`          → added to Resource.Attributes under the field name
//	`atlasent:"attr,name=foo"` → added under the explicit attribute name "foo"
//	`atlasent:"-"`             → skipped
//
// If no field carries `atlasent:"type"`, the caller can pass a
// defaultType; otherwise the function returns an error. Pointer and slice
// fields are dereferenced; nil pointers become empty Attributes entries.
//
// This is a convenience for codebases that model domain types; for
// ad-hoc code `atlasent.Resource{...}` remains the clearer construction.
func ResourceFrom(v any, defaultType string) (Resource, error) {
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return Resource{}, fmt.Errorf("atlasent: ResourceFrom: nil pointer")
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return Resource{}, fmt.Errorf("atlasent: ResourceFrom: want struct, got %s", rv.Kind())
	}
	rt := rv.Type()

	res := Resource{Type: defaultType}
	var attrs map[string]any

	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if !f.IsExported() {
			continue
		}
		tag, ok := f.Tag.Lookup("atlasent")
		if !ok {
			continue
		}
		if tag == "-" {
			continue
		}
		parts := strings.Split(tag, ",")
		role := parts[0]
		options := parts[1:]

		switch role {
		case "id":
			res.ID = asString(rv.Field(i))
		case "type":
			res.Type = asString(rv.Field(i))
		case "attr":
			name := f.Name
			for _, o := range options {
				if strings.HasPrefix(o, "name=") {
					name = strings.TrimPrefix(o, "name=")
				}
			}
			if attrs == nil {
				attrs = map[string]any{}
			}
			attrs[name] = rv.Field(i).Interface()
		default:
			return Resource{}, fmt.Errorf("atlasent: ResourceFrom: unknown tag role %q on field %s", role, f.Name)
		}
	}

	if res.Type == "" {
		return Resource{}, fmt.Errorf("atlasent: ResourceFrom: no type resolved (pass defaultType or tag a field `atlasent:\"type\"`)")
	}
	if attrs != nil {
		res.Attributes = attrs
	}
	return res, nil
}

func asString(v reflect.Value) string {
	switch v.Kind() {
	case reflect.String:
		return v.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fmt.Sprintf("%d", v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fmt.Sprintf("%d", v.Uint())
	case reflect.Pointer:
		if v.IsNil() {
			return ""
		}
		return asString(v.Elem())
	default:
		return fmt.Sprintf("%v", v.Interface())
	}
}
