package framework

import (
	//"github.com/zhangel/go-framework.git"
	"log"
	"sync"
)

var (
	once sync.Once
)

func init() {

}

func Finalize() {
	log.Printf("callback exec")
	//lifecycle.LifeCycle.Finalize()
}

func Init() func() {
	once.Do(func() {

	})
	return Finalize
}
