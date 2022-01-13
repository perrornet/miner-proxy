package pkg

import "fmt"

func Input(msg string, f func(s string) bool) string {
	for {
		fmt.Printf(msg)
		var s string
		fmt.Scan(&s)
		if f(s) {
			return s
		}
	}
}
