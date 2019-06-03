package gen

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

const (
	gitFilename       = ".git"
	gitignoreFilename = ".gitignore"
)

func Execute(prefix string, suffix string, noIgnore bool, dryRun bool, verbose bool, args []string) {
	if verbose {
		fmt.Printf("prefix: %s; suffix: %s; dry-run: %t\n", prefix, suffix, dryRun)
	}

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

	files := make(chan string)
	jobs := make(chan string)
	parserWg := sync.WaitGroup{}
	executorWg := sync.WaitGroup{}
	for i := 0; i < 1; i++ {
		parserWg.Add(1)
		go fileParser(files, jobs, &parserWg, prefix, suffix, verbose)
		executorWg.Add(1)
		go jobExecutor(jobs, &executorWg, dryRun)
	}
	for _, i := range filepathSet.list {
		files <- i
	}
	close(files)
	parserWg.Wait()
	close(jobs)
	executorWg.Wait()
}

func fileParser(files <-chan string, jobs chan<- string, wg *sync.WaitGroup, prefix, suffix string, verbose bool) {
	defer wg.Done()
	for {
		filename, ok := <-files
		if !ok {
			return
		}
		if verbose {
			fmt.Printf("parsing: %s\n", filename)
		}
		directives, err := parseFile(filename, prefix, suffix)
		if err != nil {
			fmt.Printf("failed reading file %s: %s\n", filename, err)
			continue
		}
		for _, i := range directives {
			jobs <- i
		}
	}
}

func parseFile(filename string, prefix, suffix string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	directives := []string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		directive, ok := parseLine(scanner.Text(), prefix, suffix)
		if ok {
			directives = append(directives, directive)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return directives, nil
}

func parseLine(line string, prefix, suffix string) (string, bool) {
	prefixLoc := strings.Index(line, prefix)
	if prefixLoc < 0 {
		return "", false
	}
	commandLoc := prefixLoc + len(prefix)
	directive := ""
	suffixLoc := strings.Index(line, suffix)
	if suffixLoc > commandLoc {
		directive = line[commandLoc:suffixLoc]
	} else {
		directive = line[commandLoc:]
	}
	return strings.TrimSpace(directive), true
}

func jobExecutor(jobs <-chan string, wg *sync.WaitGroup, dryRun bool) {
	defer wg.Done()
	for {
		job, ok := <-jobs
		if !ok {
			return
		}
		fmt.Printf("exec: %s\n", job)
		if !dryRun {
			executeJob(job)
		}
	}
}

func executeJob(job string) {
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
