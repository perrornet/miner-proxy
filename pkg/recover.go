package pkg

import (
	"log"
	"runtime/debug"
)

func Recover(showStack bool) {
	if err := recover(); err != nil {
		log.Println(err)
		if showStack {
			debug.PrintStack()
		}
	}
}
