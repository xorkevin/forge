package gen

import (
	"errors"
	"os"
	"regexp"
	"strconv"
	"strings"
)

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

const (
	tokenSpace = iota
	tokenText
	tokenStartDouble
	tokenEndDouble
	tokenStartSingle
	tokenEndSingle
)

var (
	errUnclosedDouble = errors.New("unclosed double quote")
	errUnclosedSingle = errors.New("unclosed single quote")
	errInvalidEscape  = errors.New("invalid escape")
)

type (
	token struct {
		id  int
		val string
	}

	matcher func(string) ([]token, string, error)
)

func tokenize(directive string) ([][]token, error) {
	args := [][]token{}
	for text := trimLSpace(directive); len(text) > 0; text = trimLSpace(text) {
		t, next, err := matcherArg(text)
		if err != nil {
			return nil, err
		}
		args = append(args, t)
		text = next
	}
	return args, nil
}

const (
	spaceCharSet = " \t\r\n"
)

func isSpace(c byte) bool {
	switch c {
	case ' ', '\t', '\r', '\n':
		return true
	default:
		return false
	}
}

func trimLSpace(s string) string {
	return strings.TrimLeft(s, spaceCharSet)
}

func matcherArg(text string) ([]token, string, error) {
	tokens := []token{}
	i := 0
	for i < len(text) {
		ch := text[i]
		if isSpace(ch) {
			t, next, err := matcherSpace(text[i:])
			if err != nil {
				return nil, "", err
			}
			if i > 0 {
				tokens = append(tokens, token{
					id:  tokenText,
					val: text[0:i],
				})
			}
			tokens = append(tokens, t...)
			text = next
			i = 0
			break
		} else if ch == '"' {
			t, next, err := matcherDoubleQuote(text[i:])
			if err != nil {
				return nil, "", err
			}
			if i > 0 {
				tokens = append(tokens, token{
					id:  tokenText,
					val: text[0:i],
				})
			}
			tokens = append(tokens, t...)
			text = next
			i = 0
		} else {
			i++
		}
	}
	if i > 0 {
		tokens = append(tokens, token{
			id:  tokenText,
			val: text[0:i],
		})
		text = text[i:]
		i = 0
	}
	return tokens, text, nil
}

func matcherSpace(text string) ([]token, string, error) {
	i := 0
	for i < len(text) {
		ch := text[i]
		if !isSpace(ch) {
			break
		}
		i++
	}
	return []token{
		token{
			id: tokenSpace,
		},
	}, text[i:], nil
}

func matcherDoubleQuote(text string) ([]token, string, error) {
	tokens := []token{
		token{
			id: tokenStartDouble,
		},
	}
	i := 1
	found := false
	for i < len(text) {
		ch := text[i]
		if ch == '\\' {
			if i+1 >= len(text) {
				return nil, "", errInvalidEscape
			}
			i += 2
		} else if ch == '"' {
			found = true
			if i > 0 {
				s, err := strconv.Unquote(text[0 : i+1])
				if err != nil {
					return nil, "", errStringMisquote
				}
				tokens = append(tokens, token{
					id:  tokenText,
					val: s,
				})
			}
			tokens = append(tokens, token{
				id: tokenEndDouble,
			})
			text = text[i+1:]
			i = 0
			break
		} else {
			i++
		}
	}
	if !found {
		return nil, "", errUnclosedDouble
	}
	return tokens, text, nil
}

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
