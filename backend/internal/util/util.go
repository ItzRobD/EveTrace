package util

import (
	"fmt"
	"reflect"
)

func PrintStructFields(v interface{}, prefix string) {
	val := reflect.ValueOf(v)
	typ := val.Type()

	// Handle pointer to struct
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
		typ = val.Type()
	}

	// Ensure the input is a struct
	if typ.Kind() == reflect.Struct {
		for i := 0; i < val.NumField(); i++ {
			field := val.Field(i)
			fieldType := typ.Field(i)
			fieldName := fieldType.Name

			// Check if the field is an embedded struct
			if fieldType.Anonymous {
				// Recursive call for embedded struct
				PrintStructFields(field.Interface(), prefix+fieldName+".")
			} else {
				fmt.Printf("%s%s: %v\n", prefix, fieldName, field.Interface())
			}
		}
	} else {
		fmt.Println("Provided value is not a struct")
	}
}
