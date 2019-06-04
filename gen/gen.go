package gen

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	envforgepath = "FORGEPATH"
	envforgefile = "FORGEFILE"
	envforgeline = "FORGELINE"
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
	errNotUTF8        = errors.New("not a utf8 file")
	errStringMisquote = errors.New("misquoted string")
	errUnclosedString = errors.New("unclosed string")
	errUnclosedBrace  = errors.New("unclosed brace")
	errInvalidEnvVar  = errors.New("invalid env var name")
	errNoPrefix       = errors.New("no prefix")
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

	for _, filepath := range filepathSet.list {
		if verbose {
			fmt.Printf("parsing: %s\n", filepath)
		}
		directives, filename, err := parseFile(filepath, prefix, suffix)
		if err != nil {
			if err == errNotUTF8 {
				if verbose {
					fmt.Printf("ignoring %s: not a utf8 file\n", filepath)
				}
				continue
			}
			fmt.Printf("failed parsing file %s: %s\n", filepath, err)
			continue
		}

		fileenv := append(environ, envforgepath+"="+filepath, envforgefile+"="+filename)
		for _, i := range directives {
			fmt.Printf("forge exec: %s line %d: %s\n", filepath, i.line, i.text)
			if !dryRun {
				if err := executeJob(i.args, append(fileenv, envforgeline+"="+strconv.Itoa(i.line))); err != nil {
					fmt.Printf("failed: %s", err)
				}
			}
		}
	}
}

type (
	directive struct {
		line int
		args []string
		text string
	}
)

func parseFile(filepath string, prefix, suffix string) ([]directive, string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, "", err
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Fatal(err)
		}
	}()
	filename := file.Name()

	directives := []directive{}
	scanner := bufio.NewScanner(file)
	for i := 0; scanner.Scan(); i++ {
		text := scanner.Text()
		if !utf8.ValidString(text) {
			return nil, "", errNotUTF8
		}
		forgeenv := map[string]string{
			envforgepath: filepath,
			envforgefile: filename,
			envforgeline: strconv.Itoa(i),
		}
		args, text, err := parseLine(text, prefix, suffix, forgeenv)
		if err != nil {
			if err == errNoPrefix {
				continue
			}
			return nil, "", fmt.Errorf("line %d: %s", i, err)
		}
		directives = append(directives, directive{
			line: i,
			args: args,
			text: text,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, "", err
	}
	return directives, filename, nil
}

func parseLine(line string, prefix, suffix string, forgeenv map[string]string) ([]string, string, error) {
	prefixLoc := strings.Index(line, prefix)
	if prefixLoc < 0 {
		return nil, "", errNoPrefix
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
	args, err := parseArgs(directive, forgeenv)
	if err != nil {
		return nil, "", err
	}
	return args, directive, nil
}

func parseArgs(directive string, forgeenv map[string]string) ([]string, error) {
	args := []string{}
	for text := directive; len(text) > 0; text = strings.TrimLeft(text, " \t") {
		replace := true
		strmode := false
		doublequote := true

		switch text[0] {
		case '\'':
			replace = false
			strmode = true
			doublequote = false
		case '"':
			strmode = true
		}

		var arg string
		if strmode {
			found := false
			for i := 1; i < len(text); i++ {
				ch := text[i]
				if ch == byte('\\') {
					i++
					continue
				}
				if doublequote && ch == '"' || !doublequote && ch == '\'' {
					found = true
					k := i + 1
					if doublequote {
						a, err := strconv.Unquote(text[0:k])
						if err != nil {
							return nil, errStringMisquote
						}
						arg = a
					} else {
						arg = text[1:i]
					}
					text = text[k:]
					break
				}
			}
			if !found {
				return nil, errUnclosedString
			}
		} else {
			k := strings.IndexAny(text, " \t")
			if k < 0 {
				k = len(text)
			}
			arg = text[0:k]
			text = text[k:]
			if strings.Contains(arg, "${") {
				i := strings.IndexAny(text, "}")
				if i < 0 {
					return nil, errUnclosedBrace
				}
				k = i + 1
				arg += text[0:k]
				text = text[k:]
				k = strings.IndexAny(text, " \t")
				if k < 0 {
					k = len(text)
				}
				arg += text[0:k]
				text = text[k:]
			}
		}

		if replace {
			a, err := replaceEnvVar(arg, forgeenv)
			if err != nil {
				return nil, err
			}
			arg = a
		}
		args = append(args, arg)
	}
	return args, nil
}

func executeJob(args []string, env []string) error {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env
	return cmd.Run()
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
