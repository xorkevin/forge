package gen

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	gitFilename       = ".git"
	gitignoreFilename = ".gitignore"
)

func Execute(prefix string, suffix string, noIgnore bool, dryRun bool, verbose bool, args []string) {
	workingDir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	environ := append(os.Environ(), "FORGEDIR="+workingDir)

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

	for _, filename := range filepathSet.list {
		if verbose {
			fmt.Printf("parsing: %s\n", filename)
		}
		directives, err := parseFile(filename, prefix, suffix)
		if err != nil {
			fmt.Printf("failed reading file %s: %s\n", filename, err)
			continue
		}

		fileenv := append(environ, "FORGEFILE="+filename)
		for _, i := range directives {
			fmt.Printf("forge exec: %s line %d: %s\n", filename, i.line, i.text)
			if !dryRun {
				if err := executeJob(i.job, append(fileenv, "FORGELINE="+strconv.Itoa(i.line))); err != nil {
					fmt.Printf("failed: %s", err)
				}
			}
		}
	}
}

type (
	directive struct {
		line int
		job  []string
		text string
	}
)

func parseFile(filename string, prefix, suffix string) ([]directive, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	directives := []directive{}
	scanner := bufio.NewScanner(file)
	for i := 0; scanner.Scan(); i++ {
		job, text, ok := parseLine(scanner.Text(), prefix, suffix)
		if ok {
			directives = append(directives, directive{
				line: i,
				job:  job,
				text: text,
			})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return directives, nil
}

func parseLine(line string, prefix, suffix string) ([]string, string, bool) {
	prefixLoc := strings.Index(line, prefix)
	if prefixLoc < 0 {
		return nil, "", false
	}
	commandLoc := prefixLoc + len(prefix)
	directive := ""
	suffixLoc := strings.Index(line, suffix)
	if suffixLoc > commandLoc {
		directive = line[commandLoc:suffixLoc]
	} else {
		directive = line[commandLoc:]
	}
	directive = strings.TrimSpace(directive)
	return strings.Fields(directive), directive, true
}

func executeJob(job []string, env []string) error {
	cmd := exec.Command(job[0], job[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env
	return cmd.Run()
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
