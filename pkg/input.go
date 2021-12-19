package pkg

import "fmt"

func Input(message string, f func(s string) bool) {
	for {
		fmt.Printf(message)
		var s string
		fmt.Scanln(&s)
		if !f(s) {
			continue
		}
		return
	}
}
