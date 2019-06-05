package gen

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_matcherArg(t *testing.T) {
	assert := assert.New(t)

	{
		arg := `hello"world"`
		tokens, next, err := matcherArg(arg)
		assert.NoError(err, "matcher should not error")
		assert.Equal("", next, "all spaces should be parsed")
		tokenization := []token{
			token{tokenText, "hello"},
			token{tokenStartDouble, ""},
			token{tokenText, "world"},
			token{tokenEndDouble, ""},
		}
		assert.Equal(tokenization, tokens, "all parts of the input must be tokenized")
	}
	{
		arg := `hello"world"   `
		tokens, next, err := matcherArg(arg)
		assert.NoError(err, "matcher should not error")
		assert.Equal("", next, "all spaces should be parsed")
		tokenization := []token{
			token{tokenText, "hello"},
			token{tokenStartDouble, ""},
			token{tokenText, "world"},
			token{tokenEndDouble, ""},
			token{tokenSpace, ""},
		}
		assert.Equal(tokenization, tokens, "all parts of the input must be tokenized")
	}
	{
		arg := `"hello"world arg2 `
		tokens, next, err := matcherArg(arg)
		assert.NoError(err, "matcher should not error")
		assert.Equal("arg2 ", next, "all spaces should be parsed")
		tokenization := []token{
			token{tokenStartDouble, ""},
			token{tokenText, "hello"},
			token{tokenEndDouble, ""},
			token{tokenText, "world"},
			token{tokenSpace, ""},
		}
		assert.Equal(tokenization, tokens, "all parts of the input must be tokenized")
	}
	{
		arg := `"hello"world`
		tokens, next, err := matcherArg(arg)
		assert.NoError(err, "matcher should not error")
		assert.Equal("", next, "all spaces should be parsed")
		tokenization := []token{
			token{tokenStartDouble, ""},
			token{tokenText, "hello"},
			token{tokenEndDouble, ""},
			token{tokenText, "world"},
		}
		assert.Equal(tokenization, tokens, "all parts of the input must be tokenized")
	}
	{
		arg := `"hello\"world"`
		tokens, next, err := matcherArg(arg)
		assert.NoError(err, "matcher should not error")
		assert.Equal("", next, "all spaces should be parsed")
		tokenization := []token{
			token{tokenStartDouble, ""},
			token{tokenText, "hello\"world"},
			token{tokenEndDouble, ""},
		}
		assert.Equal(tokenization, tokens, "all parts of the input must be tokenized")
	}
	{
		arg := `"hello\"world`
		_, _, err := matcherArg(arg)
		assert.Equal(errUnclosedDouble, err, "matcher should error on unclosed double quotes")
	}
	{
		arg := `"hello\"world\`
		_, _, err := matcherArg(arg)
		assert.Equal(errInvalidEscape, err, "matcher should error on invalid escapes")
	}
}
