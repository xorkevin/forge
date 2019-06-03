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
	"regexp"
	"strconv"
	"strings"
)

const (
	gitFilename       = ".git"
	gitignoreFilename = ".gitignore"
	envforgepath      = "FORGEPATH"
	envforgefile      = "FORGEFILE"
	envforgeline      = "FORGELINE"
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

	for _, filepath := range filepathSet.list {
		if verbose {
			fmt.Printf("parsing: %s\n", filepath)
		}
		directives, filename, err := parseFile(filepath, prefix, suffix)
		if err != nil {
			fmt.Printf("failed reading file %s: %s\n", filepath, err)
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
		args, text, ok := parseLine(scanner.Text(), prefix, suffix, filepath, filename, i)
		if !ok {
			continue
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

func parseLine(line string, prefix, suffix string, filepath, filename string, lineno int) ([]string, string, bool) {
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
	args, err := parseArgs(directive, filename, filepath, lineno)
	if err != nil {
		fmt.Printf("failed parsing %s line %d: %s\n", filepath, lineno, err)
		return nil, "", false
	}
	return args, directive, true
}

func parseArgs(directive string, filepath, filename string, lineno int) ([]string, error) {
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
							return nil, errors.New("misquoted string")
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
				return nil, errors.New("unclosed quote")
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
					return nil, errors.New("unclosed brace")
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
			a, err := replaceEnvVar(arg, filepath, filename, lineno)
			if err != nil {
				return nil, err
			}
			arg = a
		}
		args = append(args, arg)
	}
	return args, nil
}

const (
	envvardefaultSeparator = ":-"
)

var (
	regexAlphanum = regexp.MustCompile("^[a-zA-Z_][a-zA-Z0-9_]*$")
)

func replaceEnvVar(arg string, filepath, filename string, lineno int) (string, error) {
	s := strings.Builder{}
	for text := arg; len(text) > 0; {
		k := strings.IndexAny(text, "$")
		if k < 0 || k+1 >= len(text) {
			s.WriteString(text)
			break
		}
		s.WriteString(text[0:k])
		text = text[k+1:]

		envvar := ""
		envvaldefault := ""
		if regexAlphanum.MatchString(string(text[0])) {
			end := strings.IndexAny(text, " \t")
			if end < 0 {
				end = len(text)
			}
			envvar = text[0:end]
			text = text[end:]
		} else if text[0] == '{' {
			text = text[1:]
			end := strings.IndexAny(text, "}")
			if end < 0 {
				return "", errors.New("unclosed brace")
			}
			envpair := strings.SplitN(text[0:end], envvardefaultSeparator, 2)
			envvar = envpair[0]
			if len(envpair) > 1 {
				envvaldefault = envpair[1]
			}
			text = text[end+1:]
		} else {
			s.WriteString("$")
			continue
		}
		if !regexAlphanum.MatchString(envvar) {
			return "", errors.New("invalid env var name")
		}

		s.WriteString(lookupEnv(envvar, envvaldefault, filepath, filename, lineno))
	}
	return s.String(), nil
}

func lookupEnv(envvar string, envvaldefault string, filepath, filename string, lineno int) string {
	switch envvar {
	case envforgepath:
		return filepath
	case envforgefile:
		return filename
	case envforgeline:
		return strconv.Itoa(lineno)
	default:
		if val, ok := os.LookupEnv(envvar); ok {
			return val
		} else {
			return envvaldefault
		}
	}
}

func executeJob(args []string, env []string) error {
	cmd := exec.Command(args[0], args[1:]...)
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
