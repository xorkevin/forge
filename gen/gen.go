package gen

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"github.com/hackform/nutcracker"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const (
	envforgepath = "FORGEPATH"
	envforgefile = "FORGEFILE"
)

var (
	noTrackDirnames  *stringSet
	noTrackFilenames *stringSet
)

func init() {
	dirnames := []string{
		".git",
	}
	noTrackDirnames = newStringSet(len(dirnames))
	for _, i := range dirnames {
		noTrackDirnames.add(i)
	}
}

func init() {
	filenames := []string{
		".gitignore",
		".gitmodules",
	}
	noTrackFilenames = newStringSet(len(filenames))
	for _, i := range filenames {
		noTrackFilenames.add(i)
	}
}

var (
	errNotUTF8 = errors.New("not a utf8 file")
)

func Execute(prefix string, suffix string, noIgnore bool, dryRun bool, verbose bool, args []string) {
	if len(prefix) == 0 {
		log.Fatalln("directive prefix cannot be empty")
	}
	if len(suffix) == 0 {
		log.Fatalln("directive suffix cannot be empty")
	}
	workingDir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	environ := append(os.Environ(), "FORGEDIR="+workingDir)

	if verbose {
		fmt.Printf("prefix: %s; suffix: %s\n", prefix, suffix)
	}

	paths := []string{"."}
	if len(args) > 0 {
		paths = args
	}

	ignorePaths := newStringSet(0)
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

	ex := nutcracker.NewExecutor()

	filepathSet := newStringSet(0)
	for _, i := range paths {
		if err := filepath.Walk(i, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// do not track blacklisted dirs and files
			filename := info.Name()
			if info.IsDir() {
				if noTrackDirnames.has(filename) {
					return filepath.SkipDir
				}
			} else {
				if noTrackFilenames.has(filename) {
					return nil
				}
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

	splitDir := makeSplitDirective(prefix, suffix)

	for _, fpath := range filepathSet.list {
		if verbose {
			fmt.Printf("parsing: %s\n", fpath)
		}
		directives, err := parseFile(fpath, splitDir)
		if err != nil {
			if err == errNotUTF8 {
				if verbose {
					fmt.Printf("ignoring %s: not a utf8 file\n", fpath)
				}
				continue
			}
			fmt.Printf("failed parsing file %s: %s\n", fpath, err)
			continue
		}

		filename := filepath.Base(fpath)
		envvar := nutcracker.Env{
			Envvar: append(environ, envforgepath+"="+fpath, envforgefile+"="+filename),
			Envfunc: func(name string) string {
				switch name {
				case envforgepath:
					return fpath
				case envforgefile:
					return filename
				default:
					val, _ := os.LookupEnv(name)
					return val
				}
			},
			Stdout: os.Stdout,
			Stderr: os.Stderr,
			Ex:     ex,
		}
		for _, i := range directives {
			fmt.Printf("forge exec: %s count %d: %s\n", fpath, i.count, i.text)
			if !dryRun {
				if err := i.cmd.Exec(envvar); err != nil {
					fmt.Printf("failed: %s", err)
				}
			}
		}
	}
	if dryRun {
		fmt.Printf("dry-run: %t\n", dryRun)
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func makeSplitDirective(prefix, suffix string) bufio.SplitFunc {
	prefixBytes := []byte(prefix)
	suffixBytes := []byte(suffix)
	prefixLen := len(prefixBytes)
	suffixLen := len(suffixBytes)

	return func(data []byte, atEOF bool) (int, []byte, error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}
		if !utf8.Valid(data) {
			return 0, nil, errNotUTF8
		}

		prefixLoc := bytes.Index(data, prefixBytes)

		if prefixLoc < 0 {
			if atEOF {
				// no directive present, read until end of file
				return len(data), nil, nil
			}

			// save len(prefix)-1 bytes since they might contain prefix beginning
			return maxInt(len(data)-prefixLen+1, 0), nil, nil
		}

		suffixLoc := bytes.Index(data, suffixBytes)

		if suffixLoc < prefixLoc+prefixLen {
			return prefixLoc, nil, nil
		}

		return suffixLoc + suffixLen, data[prefixLoc+prefixLen : suffixLoc], nil
	}
}

type (
	directive struct {
		count int
		cmd   *nutcracker.Cmd
		text  string
	}
)

func parseFile(fpath string, splitDirective bufio.SplitFunc) ([]directive, error) {
	file, err := os.Open(fpath)
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
	scanner.Split(splitDirective)
	for i := 0; scanner.Scan(); i++ {
		cmd, text, err := parseDirective(scanner.Text())
		if err != nil {
			return nil, fmt.Errorf("count %d: %s", i, err)
		}
		directives = append(directives, directive{
			count: i,
			cmd:   cmd,
			text:  text,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return directives, nil
}

func parseDirective(directive string) (*nutcracker.Cmd, string, error) {
	directive = strings.TrimSpace(directive)
	cmd, err := nutcracker.Parse(directive)
	if err != nil {
		return nil, "", err
	}
	return cmd, strings.ReplaceAll(directive, "\n", "\\n"), nil
}

func generateIgnorePathSet() (*stringSet, error) {
	cmd := exec.Command("git", "ls-files", "-oi", "--directory", "--exclude-standard")
	out := bytes.Buffer{}
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return newStringSet(0), err
	}
	ignorePathBytes := bytes.Split(bytes.Trim(out.Bytes(), "\n"), []byte{'\n'})

	cmd = exec.Command("git", "submodule", "-q", "foreach", "--recursive", "echo $displaypath")
	out = bytes.Buffer{}
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return newStringSet(0), err
	}
	submodulePathBytes := bytes.Split(bytes.Trim(out.Bytes(), "\n"), []byte{'\n'})

	ignorePaths := newStringSet(len(ignorePathBytes))
	for _, i := range ignorePathBytes {
		k := string(i)
		if len(k) == 0 {
			continue
		}
		k = filepath.Clean(string(i))
		ignorePaths.add(k)
	}
	for _, i := range submodulePathBytes {
		k := string(i)
		if len(k) == 0 {
			continue
		}
		k = filepath.Clean(string(i))
		ignorePaths.add(k)
	}
	return ignorePaths, nil
}

type (
	stringSet struct {
		set  map[string]struct{}
		list []string
	}
)

func newStringSet(size int) *stringSet {
	return &stringSet{
		set:  make(map[string]struct{}, size),
		list: make([]string, 0, size),
	}
}

func (ps *stringSet) add(path string) bool {
	if ps.has(path) {
		return false
	}
	ps.set[path] = struct{}{}
	ps.list = append(ps.list, path)
	return true
}

func (ps stringSet) has(path string) bool {
	_, ok := ps.set[path]
	return ok
}
