package gen

import (
	"os"
	"regexp"
	"strings"
)

const (
	envvardefaultSeparator = ":-"
)

var (
	regexAlphanum = regexp.MustCompile("^[a-zA-Z_][a-zA-Z0-9_]*$")
)

func replaceEnvVar(arg string, forgeenv map[string]string) (string, error) {
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
				return "", errUnclosedBrace
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
			return "", errInvalidEnvVar
		}

		s.WriteString(lookupEnv(envvar, envvaldefault, forgeenv))
	}
	return s.String(), nil
}

func lookupEnv(envvar string, envvaldefault string, forgeenv map[string]string) string {
	if val, ok := forgeenv[envvar]; ok {
		return val
	}
	if val, ok := os.LookupEnv(envvar); ok {
		return val
	}
	return envvaldefault
}
