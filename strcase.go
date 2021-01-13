package rockhopper

import (
	"bytes"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

type camelSnakeStateMachine int

const ( //                                           _$$_This is some text, OK?!
	idle          camelSnakeStateMachine = iota // 0 ↑                     ↑   ↑
	firstAlphaNum                               // 1     ↑    ↑  ↑    ↑     ↑
	alphaNum                                    // 2      ↑↑↑  ↑  ↑↑↑  ↑↑↑   ↑
	delimiter                                   // 3         ↑  ↑    ↑    ↑   ↑
)

func (s camelSnakeStateMachine) next(r rune) camelSnakeStateMachine {
	switch s {
	case idle:
		if isAlphaNum(r) {
			return firstAlphaNum
		}
	case firstAlphaNum:
		if isAlphaNum(r) {
			return alphaNum
		}
		return delimiter
	case alphaNum:
		if !isAlphaNum(r) {
			return delimiter
		}
	case delimiter:
		if isAlphaNum(r) {
			return firstAlphaNum
		}
		return idle
	}
	return s
}

func snakeCase(str string) string {
	var b bytes.Buffer

	stateMachine := idle
	for i := 0; i < len(str); {
		r, size := utf8.DecodeRuneInString(str[i:])
		i += size
		stateMachine = stateMachine.next(r)
		switch stateMachine {
		case firstAlphaNum, alphaNum:
			b.WriteRune(unicode.ToLower(r))
		case delimiter:
			b.WriteByte('_')
		}
	}
	if stateMachine == idle {
		return string(bytes.TrimSuffix(b.Bytes(), []byte{'_'}))
	}
	return b.String()
}

func isAlphaNum(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r)
}

var snakeCasePattern = regexp.MustCompile("[_\\s]+[a-z]+")

func toCamelCase(s string) string {
	return snakeCasePattern.ReplaceAllStringFunc(s, func(s string) string {
		return strings.Title(strings.TrimLeft(s, "_ "))
	})
}

