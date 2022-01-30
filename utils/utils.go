package utils

import (
	"fmt"
	"reflect"
)

func PrintMethods(obj interface{}) {
	value := reflect.ValueOf(obj)
	_type := value.Type()
	for i := 0; i < _type.NumMethod(); i++ {
		fmt.Printf("%s\t\t%+v\n", _type.Method(i).Name, _type.Method(i).Type)
	}
}
