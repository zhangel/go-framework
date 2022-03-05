package registry

import "encoding/json"

const (
	//CurrentVersion 当前注册协议版本
	/**
	 * Version 1:
	 * 	 初始版本，GRPC支持，每个HttpEndPoint一个独立注册项
	 * Version 2:
	 * 	 HttpPatterns合并为一个注册项；注册项包含TLS信息和服务类型信息
	 * Version 3:
	 *   GRPC注册部分没有变化，Http注册部分增加HttpRules，支持HTTP-Header匹配从而解决Pattern一致情况下的冲突问题，后续可以继续增加其他匹配类型
	 *   暂时保留HttpPatterns，用于向前兼容，但HttpPatterns在后续版本可能废弃
	 */

	CurrentVersion = 3

	RegistryType_Grpc          = 1
	RegistryType_Http          = 2
	RegistryType_Http_Endpoint = 3
)

type HttpRule struct {
	HttpPatterns   []string            `json:"http,omitempty"`
	HeaderMap      []map[string]string `json:"header_map,omitempty"`
	HeaderRegexMap []map[string]string `json:"header_regex_map,omitempty"`
}

type Meta struct {
	TlsServer    bool        `json:"tls"`
	HttpPatterns []string    `json:"http,omitempty"`
	Version      int         `json:"version"`
	Type         int         `json:"type"`
	HttpRules    []*HttpRule `json:"http_rules,omitempty"`
}

func MarshalRegistryMeta(registryMeta Meta) (string, error) {
	registryMeta.Version = CurrentVersion
	if metaJson, err := json.Marshal(registryMeta); err != nil {
		return "", err
	} else {
		return string(metaJson), nil
	}
}

func UnmarshalRegistryMeta(jsonMeta string) (Meta, error) {
	registryMeta := Meta{}
	if err := json.Unmarshal([]byte(jsonMeta), &registryMeta); err != nil {
		return registryMeta, err
	} else {
		if registryMeta.Version == 0 {
			registryMeta.Version = 1
		}
		return registryMeta, nil
	}
}
