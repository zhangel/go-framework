package framework

import (
	//"github.com/zhangel/go-framework.git"
	"log"
	"sync"
)

var (
	once      sync.Once
	presetOpt []Option
)

func init() {

}

func Finalize() {
	log.Printf("callback exec")
	//lifecycle.LifeCycle.Finalize()
}

func Init(opt ...Option) func() {
	once.Do(func() {
		opts := Options{}
		for _, o := range append(presetOpt, opt...) {
			if err := o(opt); err != nil {
				log.Fatal("[LOAD_ERROR]", opt)
			}
		}
		log.Printf("opts=%+v\n", opts)

	})
	return Finalize
}
