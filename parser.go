package rockhopper

import (
	"bufio"
	"bytes"
	"io"
	"regexp"
	"strings"
	"sync"

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
	Direction Direction `json:"direction" yaml:"direction"`
	UseTx     bool      `json:"useTxn" yaml:"useTxn"`
	SQL       string    `json:"sql" yaml:"sql"`
}

type MigrationParser struct {
	bufferPool *sync.Pool
}

func (p *MigrationParser) ParseBytes(data []byte) (upStmts, downStmts []Statement, err error) {
	buf := bytes.NewBuffer(data)
	return p.Parse(buf)
}

func (p *MigrationParser) ParseString(data string) (upStmts, downStmts []Statement, err error) {
	buf := bytes.NewBufferString(data)
	return p.Parse(buf)
}

func (p *MigrationParser) Parse(r io.Reader) (upStmts, downStmts []Statement, err error) {
	if p.bufferPool == nil {
		p.bufferPool = &sync.Pool{
			New: func() interface{} {
				return make([]byte, scanBufSize)
			},
		}
	}

	var buf bytes.Buffer
	scanBuf := p.bufferPool.Get().([]byte)
	defer p.bufferPool.Put(scanBuf)

	scanner := bufio.NewScanner(r)
	scanner.Buffer(scanBuf, scanBufSize)

	var state = start
	var useTx = true

	for scanner.Scan() {
		line := scanner.Text()

		var isEnd = false
		if strings.HasPrefix(line, "--") {
			cmd := strings.TrimSpace(strings.TrimPrefix(line, "--"))

			// make it goose compatible, replacing +goose Up to just +up
			cmd = strings.ToLower(strings.Replace(cmd, "+goose ", "+", -1))

			switch cmd {
			case "+up":
				switch state {
				case start:
					state = stateUp
				default:
					return nil, nil, errors.Errorf("duplicate '-- +up' annotations; state=%v, see https://github.com/c9s/goose#sql-migrations", state)
				}
				continue

			case "+down":
				switch state {
				case stateUp, stateUpStatementEnd:
					state = stateDown
				default:
					err = errors.Errorf("must start with '-- +up' annotation, state=%v", state)
				}
				continue

			case "+begin":
				switch state {
				case stateUp, stateUpStatementEnd:
					state = stateUpStatementBegin
				case stateDown, stateDownStatementEnd:
					state = stateDownStatementBegin
				default:
					err = errors.Errorf("'-- +begin' must be defined after '-- +up' or '-- +down' annotation, state=%v, see https://github.com/c9s/goose#sql-migrations", state)
					return
				}

				continue

			case "+end":
				switch state {
				case stateUpStatementBegin:
					state = stateUpStatementEnd
				case stateDownStatementBegin:
					state = stateDownStatementEnd
				default:
					err = errors.New("'-- +end' must be defined after '-- +begin', see https://github.com/c9s/goose#sql-migrations")
					return
				}

				isEnd = true

			case "!txn":
				useTx = false
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
			if _, err = buf.WriteString(line + "\n"); err != nil {
				err = errors.Wrap(err, "failed to write to buf")
				return
			}
		}

		switch state {
		case stateUp:
			if p.endsWithSemicolon(line) {
				upStmts = append(upStmts, Statement{
					Direction: DirectionUp,
					UseTx:     useTx,
					SQL:       buf.String(),
				})
				buf.Reset()
			}

		case stateDown:
			if p.endsWithSemicolon(line) {
				downStmts = append(downStmts, Statement{
					Direction: DirectionDown,
					UseTx:     useTx,
					SQL:       buf.String(),
				})
				buf.Reset()
			}

		case stateUpStatementEnd:
			upStmts = append(upStmts, Statement{
				Direction: DirectionUp,
				UseTx:     useTx,
				SQL:       buf.String(),
			})
			buf.Reset()
			state = stateUp

		case stateDownStatementEnd:
			downStmts = append(downStmts, Statement{
				Direction: DirectionDown,
				UseTx:     useTx,
				SQL:       buf.String(),
			})
			buf.Reset()
			state = stateDown
		}
	} // end of for

	if err = scanner.Err(); err != nil {
		err = errors.Wrap(err, "failed to scan migration")
		return
	}
	// EOF

	switch state {
	case start:
		err = errors.New("failed to parse migration: must start with '-- +up' annotation, see https://github.com/c9s/goose#sql-migrations")
		return

	case stateUpStatementBegin, stateDownStatementBegin:
		err = errors.New("failed to parse migration: missing '-- +end' annotation")
		return
	}

	if bufferRemaining := strings.TrimSpace(buf.String()); len(bufferRemaining) > 0 {
		err = errors.Errorf("failed to parse migration: state %q, unexpected unfinished SQL query: %q: missing semicolon?", state, bufferRemaining)
		return
	}

	return
}

// Checks the line to see if the line has a statement-ending semicolon
// or if the line contains a double-dash comment.
func (p *MigrationParser) endsWithSemicolon(line string) bool {
	scanBuf := p.bufferPool.Get().([]byte)
	defer p.bufferPool.Put(scanBuf)

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
