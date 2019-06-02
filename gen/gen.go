package gen

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	gitFilename       = ".git"
	gitignoreFilename = ".gitignore"
)

func Execute(prefix string, suffix string, noIgnore bool, dryRun bool, verbose bool, args []string) {
	paths := []string{"."}
	if len(args) > 0 {
		paths = args
	}

	ignorePaths := newPathSet(0)
	if !noIgnore {
		ip, err := generateIgnorePathSet()
		if err != nil {
			if verbose {
				fmt.Printf("git ls-files error: %s\n", err)
			}
		} else {
			ignorePaths = ip
		}
	}

	if verbose {
		fmt.Println("ignored paths:")
		for _, i := range ignorePaths.list {
			fmt.Println("-", i)
		}
	}

	filepathSet := newPathSet(0)
	for _, i := range paths {
		if err := filepath.Walk(i, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// do not track .git or .gitignore files
			if filename := info.Name(); filename == gitFilename || filename == gitignoreFilename {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			// do not track ignored files
			if ignorePaths.has(path) {
				if verbose {
					fmt.Printf("ignoring: %s\n", path)
				}
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if info.Mode().IsRegular() {
				filepathSet.add(path)
			}
			return nil
		}); err != nil {
			log.Fatalf("failed reading file: %s", err.Error())
		}
	}
	for _, i := range filepathSet.list {
		if verbose {
			fmt.Printf("parsing: %s\n", i)
		}
	}
}

func generateIgnorePathSet() (*pathSet, error) {
	cmd := exec.Command("git", "ls-files", "-oi", "--directory", "--exclude-standard")
	out := bytes.Buffer{}
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return newPathSet(0), err
	}
	ignorePathBytes := bytes.Split(bytes.Trim(out.Bytes(), "\n"), []byte{'\n'})
	ignorePaths := newPathSet(len(ignorePathBytes))
	for _, i := range ignorePathBytes {
		k := filepath.Clean(string(i))
		ignorePaths.add(k)
	}
	return ignorePaths, nil
}

type (
	pathSet struct {
		set  map[string]struct{}
		list []string
	}
)

func newPathSet(size int) *pathSet {
	return &pathSet{
		set:  make(map[string]struct{}, size),
		list: make([]string, 0, size),
	}
}

func (ps *pathSet) add(path string) bool {
	if ps.has(path) {
		return false
	}
	ps.set[path] = struct{}{}
	ps.list = append(ps.list, path)
	return true
}

func (ps pathSet) has(path string) bool {
	_, ok := ps.set[path]
	return ok
}
