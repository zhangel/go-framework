package internal

import (
	_ "unsafe"
)

const GO_FRAMEWORK_TAG = "go-framework-in-process"

type PeerAddr struct{}

func (s PeerAddr) Network() string {
	return GO_FRAMEWORK_TAG
}

func (s PeerAddr) String() string {
	return GO_FRAMEWORK_TAG
}

type AuthInfo struct{}

func (s AuthInfo) AuthType() string {
	return GO_FRAMEWORK_TAG
}
