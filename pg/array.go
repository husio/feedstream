package pg

import (
	"bytes"
	"database/sql/driver"
	"errors"
	"fmt"
	"regexp"
	"strconv"
)

var (
	// unquoted array values must not contain: (" , \ { } whitespace NULL)
	// and must be at least one char
	unquotedChar  = `[^",\\{}\s(NULL)]`
	unquotedValue = fmt.Sprintf("(%s)+", unquotedChar)

	// quoted array values are surrounded by double quotes, can be any
	// character except " or \, which must be backslash escaped:
	quotedChar  = `[^"\\]|\\"|\\\\`
	quotedValue = fmt.Sprintf("\"(%s)*\"", quotedChar)

	// an array value may be either quoted or unquoted:
	arrayValue = fmt.Sprintf("(?P<value>(%s|%s))", unquotedValue, quotedValue)

	// Array values are separated with a comma IF there is more than one value:
	arrayExp = regexp.MustCompile(fmt.Sprintf("((%s)(,)?)", arrayValue))

	valueIndex int
)

func init() {
	for i, subexp := range arrayExp.SubexpNames() {
		if subexp == "value" {
			valueIndex = i
			break
		}
	}
}

type StringSlice []string

func (s *StringSlice) Scan(src interface{}) error {
	b, ok := src.([]byte)
	if !ok {
		return errors.New("scan source not []byte")
	}
	(*s) = StringSlice(parseArray(b))
	return nil
}

// Parse the output string from the array type.
//
// Regex used: (((?P<value>(([^",\\{}\s(NULL)])+|"([^"\\]|\\"|\\\\)*")))(,)?)
func parseArray(b []byte) []string {
	results := make([]string, 0)
	for _, match := range arrayExp.FindAllSubmatch(b, -1) {
		// the string might be wrapped in quotes, so trim them:
		b := bytes.Trim(match[valueIndex], "\"")
		results = append(results, string(b))
	}
	return results
}

func (s StringSlice) Value() (driver.Value, error) {
	if len(s) == 0 {
		return nil, nil
	}

	var b bytes.Buffer
	b.WriteString("{")
	last := len(s) - 1
	for i, val := range s {
		b.WriteString(strconv.Quote(val))
		if i != last {
			b.WriteString(",")
		}
	}
	b.WriteString("}")

	return b.String(), nil
}
