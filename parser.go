package rockhopper

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
)

type parserState int

const (
	start                   parserState = iota // 0
	stateUp                                    // 1
	stateUpStatementBegin                      // 2
	stateUpStatementEnd                        // 3
	stateDown                                  // 4
	stateDownStatementBegin                    // 5
	stateDownStatementEnd                      // 6
)

const scanBufSize = 4 * 1024 * 1024

var matchEmptyLines = regexp.MustCompile(`^\s*$`)

var bufPool = &sync.Pool{
	New: func() interface{} {
		return make([]byte, scanBufSize)
	},
}

type Direction int

func (d Direction) String() string {
	switch d {
	case DirectionDown:
		return "down"

	case DirectionUp:
		return "up"
	}

	return "unset"
}

const (
	DirectionUp   Direction = 1
	DirectionDown Direction = -1
)

type Statement struct {
	Direction Direction     `json:"direction" yaml:"direction"`
	SQL       string        `json:"sql" yaml:"sql"`
	Duration  time.Duration `json:"duration" yaml:"duration"`
	Line      int           `json:"line"`
	File      string        `json:"file"`
}

type MigrationScriptChunk struct {
	UpStmts, DownStmts []Statement
	UseTx              bool
	Package            string
}

type MigrationParser struct {
}

func (p *MigrationParser) ParseBytes(data []byte) (*MigrationScriptChunk, error) {
	buf := bytes.NewBuffer(data)
	return p.Parse(buf)
}

func (p *MigrationParser) ParseString(data string) (*MigrationScriptChunk, error) {
	buf := bytes.NewBufferString(data)
	return p.Parse(buf)
}

func (p *MigrationParser) Parse(r io.Reader) (*MigrationScriptChunk, error) {
	chunk := &MigrationScriptChunk{}

	var buf bytes.Buffer
	scanBuf := bufPool.Get().([]byte)
	defer bufPool.Put(scanBuf)

	scanner := bufio.NewScanner(r)
	scanner.Buffer(scanBuf, scanBufSize)

	var state = start

	chunk.UseTx = true

	for scanner.Scan() {
		line := scanner.Text()

		var isEnd = false
		if strings.HasPrefix(line, "--") {
			cmd := strings.TrimSpace(strings.TrimPrefix(line, "--"))

			// make it goose compatible, replace +goose Up to just +up
			cmd = strings.ToLower(strings.Replace(cmd, "+goose ", "+", -1))

			if strings.HasPrefix(cmd, "@package") {
				packageName, err := matchPackageName(line)
				if err != nil {
					return nil, errors.Wrapf(err, "incorrect package statement: %s", line)
				}

				chunk.Package = packageName
				continue
			}

			switch cmd {

			case "+up":
				switch state {
				case start:
					state = stateUp
				default:
					return nil, fmt.Errorf("duplicate '-- +up' annotations; state=%v, see https://github.com/c9s/goose#sql-migrations", state)
				}
				continue

			case "+down":
				switch state {
				case stateUp, stateUpStatementEnd:
					state = stateDown
				default:
					return nil, fmt.Errorf("must start with '-- +up' annotation, state=%v", state)
				}
				continue

			case "+begin":
				switch state {
				case stateUp, stateUpStatementEnd:
					state = stateUpStatementBegin
				case stateDown, stateDownStatementEnd:
					state = stateDownStatementBegin
				default:
					return nil, fmt.Errorf("'-- +begin' must be defined after '-- +up' or '-- +down' annotation, state=%v, see https://github.com/c9s/goose#sql-migrations", state)
				}

				continue

			case "+end":
				switch state {
				case stateUpStatementBegin:
					state = stateUpStatementEnd
				case stateDownStatementBegin:
					state = stateDownStatementEnd
				default:
					return nil, errors.New("'-- +end' must be defined after '-- +begin', see https://github.com/c9s/goose#sql-migrations")
				}

				isEnd = true

			case "!txn":
				chunk.UseTx = false
				continue

			default:
				// Ignore comments.
				continue
			}
		}

		// Ignore empty lines.
		if matchEmptyLines.MatchString(line) {
			continue
		}

		// Write SQL line to a buffer.
		if !isEnd {
			if _, err := buf.WriteString(line + "\n"); err != nil {
				return nil, errors.Wrap(err, "failed to write to buf")
			}
		}

		switch state {
		case stateUp:
			if p.endsWithSemicolon(line) {
				chunk.UpStmts = append(chunk.UpStmts, Statement{
					Direction: DirectionUp,
					SQL:       strings.TrimSpace(buf.String()),
				})
				buf.Reset()
			}

		case stateDown:
			if p.endsWithSemicolon(line) {
				chunk.DownStmts = append(chunk.DownStmts, Statement{
					Direction: DirectionDown,
					SQL:       strings.TrimSpace(buf.String()),
				})
				buf.Reset()
			}

		case stateUpStatementEnd:
			chunk.UpStmts = append(chunk.UpStmts, Statement{
				Direction: DirectionUp,
				SQL:       strings.TrimSpace(buf.String()),
			})
			buf.Reset()
			state = stateUp

		case stateDownStatementEnd:
			chunk.DownStmts = append(chunk.DownStmts, Statement{
				Direction: DirectionDown,
				SQL:       strings.TrimSpace(buf.String()),
			})
			buf.Reset()
			state = stateDown
		}
	} // end of for

	if err := scanner.Err(); err != nil {
		return nil, errors.Wrap(err, "failed to scan migration")
	}
	// EOF

	switch state {
	case start:
		return nil, errors.New("failed to parse migration: must start with '-- +up' annotation, see https://github.com/c9s/goose#sql-migrations")

	case stateUpStatementBegin, stateDownStatementBegin:
		return nil, errors.New("failed to parse migration: missing '-- +end' annotation")
	}

	if bufferRemaining := strings.TrimSpace(buf.String()); len(bufferRemaining) > 0 {
		return nil, errors.Errorf("failed to parse migration: state %q, unexpected unfinished SQL query: %q: missing semicolon?", state, bufferRemaining)
	}

	return chunk, nil
}

// Checks the line to see if the line has a statement-ending semicolon
// or if the line contains a double-dash comment.
func (p *MigrationParser) endsWithSemicolon(line string) bool {
	scanBuf := bufPool.Get().([]byte)
	defer bufPool.Put(scanBuf)

	prev := ""
	scanner := bufio.NewScanner(strings.NewReader(line))
	scanner.Buffer(scanBuf, scanBufSize)
	scanner.Split(bufio.ScanWords)

	for scanner.Scan() {
		word := scanner.Text()
		if strings.HasPrefix(word, "--") {
			break
		}
		prev = word
	}

	return strings.HasSuffix(prev, ";")
}

var packageNameRegExp = regexp.MustCompile("@package\\s+(\\S+)")

func matchPackageName(line string) (string, error) {
	matches := packageNameRegExp.FindStringSubmatch(line)
	if len(matches) < 2 {
		return "", errors.New("package name not found")
	}

	return matches[1], nil
}
