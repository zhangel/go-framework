package http

import (
	"strings"
)

const RegistryPrefix = "http.server@registry"

func RegistryWithPath(methods []string, servicePath string) []string {
	var result []string
	if len(methods) == 0 {
		result = []string{"/" + "*" + strings.TrimRight(servicePath, "/")}
	} else {
		for _, method := range methods {
			result = []string{"/" + method + strings.TrimRight(servicePath, "/")}
		}
	}
	return result
}

func RegistryWithGrpcPattern(pattern string) (string, error) {
	if pattern, err := TemplateConvert(pattern); err != nil {
		return "", err
	} else {
		return "/" + pattern, nil
	}
}
