package framework

import (
	"log"
	"sync"
)

var (
	once sync.Once
)

func init() {

}

func Finalize() {

}

func Init() func() {
	once.Do(func() {

	})
	return Finalize
}

/*

func main() {

	log.Println("default_format")
	log.SetFlags(log.Ldate)
	log.Println("date_format")
	log.SetFlags(log.Ltime)
	log.Println("time_format")
	log.SetFlags(log.Lmicroseconds)
	log.Println("microseconds_format")
	log.SetFlags(log.Llongfile)
	log.Println("long_file_format")
	log.SetFlags(log.Lshortfile)
	log.Println("short_file_format")
	log.SetFlags(log.Ldate | log.Ltime | log.Llongfile | log.LUTC)
	log.Println("utc_format")
}
*/
