package gen

import (
	"fmt"
	"log"
	"path/filepath"
)

func Execute(noIgnore bool, prefix string, dryRun bool, verbose bool, args []string) {
	files := []string{}
	for _, i := range args {
		m, err := filepath.Glob(i)
		if err != nil {
			log.Fatalf("path is invalid: %s", err)
		}
		files = append(files, m...)
	}
	fmt.Println(files)
}
