package framework

import (
	"log"
	"testing"
)

func TestFramework(t *testing.T) {
	defer Init()()
	log.Printf("test framework\n")
}
