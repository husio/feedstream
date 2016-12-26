package envconf

import (
	"bytes"
	"encoding"
	"errors"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// Parse load environment variables into given structure.
//
// If program's first argument is `-h`, `--help` or `help`, instead of loading
// configuration, detailed description will be printed to stdout and the
// program is terminated with zero code.
//
// Unless function successfully parsed configuration, error is printed to
// stderr and program is terminated with non zero code.
func Parse(dest interface{}) {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "-h", "--help", "help":
			info, err := Describe(dest)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Cannot print description: %s\n", err)
				exit(1)
			}
			fmt.Fprint(os.Stdout, info)
			exit(0)
		}
	}

	env := make(map[string]string)
	for _, kv := range os.Environ() {
		pair := strings.SplitN(kv, "=", 2)
		if len(pair) != 2 {
			continue
		}
		env[pair[0]] = pair[1]
	}

	err := Load(dest, env)

	if err == nil {
		return
	}

	if errs, ok := err.(ParseErrors); ok {
		fmt.Fprintln(os.Stderr, "Cannot parse configuration")
		for _, err := range errs {
			fmt.Fprintf(os.Stderr, "  %s: %s\n", err.Name, err.Err)
		}
		exit(2)
	}

	fmt.Fprintf(os.Stderr, "Cannot parse configuration: %s\n", err)
	exit(1)
}

var exit = os.Exit

// Load assign values from settings mapping into given structure.
//
// If error is caused by invalid configuration, ParseErrors is returned.
func Load(dest interface{}, settings map[string]string) error {
	s := reflect.ValueOf(dest)
	if s.Kind() != reflect.Ptr {
		return fmt.Errorf("expected pointer to struct, got %T", dest)
	}

	s = s.Elem()
	if s.Kind() != reflect.Struct {
		return fmt.Errorf("expected struct, got %T", dest)
	}

	splitList.Lock()
	defer splitList.Unlock()

	var errs ParseErrors
	tp := s.Type()
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		if !f.CanSet() {
			continue
		}
		tags := strings.Split(tp.Field(i).Tag.Get("envconf"), ",")

		var name string
		if len(tags) > 0 && tags[0] != "" {
			name = tags[0]
		} else {
			name = convertName(tp.Field(i).Name)
		}

		required := false
		if len(tags) > 1 && contains(tags[1:], "required") {
			required = true
		}

		value, ok := settings[name]
		if !ok {
			if required {
				errs = append(errs, &ParseError{
					Field: tp.Field(i).Name,
					Name:  name,
					Value: value,
					Err:   errRequired,
					Kind:  f.Kind(),
				})
			}
			continue
		}

		if f.CanAddr() {
			if cf, ok := f.Addr().Interface().(encoding.TextUnmarshaler); ok {
				if err := cf.UnmarshalText([]byte(value)); err != nil {
					errs = append(errs, &ParseError{
						Field: tp.Field(i).Name,
						Name:  name,
						Value: value,
						Err:   errRequired,
						Kind:  f.Kind(),
					})
				}
				continue
			}
		}

		switch f.Kind() {
		case reflect.String:
			f.SetString(value)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			var intValue int64
			var err error
			if value != "" {
				intValue, err = strconv.ParseInt(value, 0, f.Type().Bits())
			}
			if err != nil {
				errs = append(errs, &ParseError{
					Field: tp.Field(i).Name,
					Name:  name,
					Value: value,
					Err:   err,
					Kind:  f.Kind(),
				})
				continue
			}
			f.SetInt(intValue)
		case reflect.Bool:
			var boolValue bool
			var err error
			if value != "" {
				boolValue, err = strconv.ParseBool(value)
			}
			if err != nil {
				errs = append(errs, &ParseError{
					Field: tp.Field(i).Name,
					Name:  name,
					Value: value,
					Err:   err,
					Kind:  f.Kind(),
				})
				continue
			}
			f.SetBool(boolValue)
		case reflect.Float32, reflect.Float64:
			var floatValue float64
			var err error
			if value != "" {
				floatValue, err = strconv.ParseFloat(value, f.Type().Bits())
			}
			if err != nil {
				errs = append(errs, &ParseError{
					Field: tp.Field(i).Name,
					Name:  name,
					Value: value,
					Err:   err,
					Kind:  f.Kind(),
				})
				continue
			}
			f.SetFloat(floatValue)
		case reflect.Slice:
			if value == "" {
				continue
			}
			vals := splitList.fn(value)

			switch f.Type().Elem().Kind() {
			case reflect.String:
				for _, v := range vals {
					f.Set(reflect.Append(f, reflect.ValueOf(v)))
				}
			case reflect.Uint8: // this is []byte
				f.SetBytes([]byte(value))
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				for _, v := range vals {
					intValue, err := strconv.ParseInt(v, 0, f.Type().Elem().Bits())
					if err != nil {
						errs = append(errs, &ParseError{
							Field: tp.Field(i).Name,
							Name:  name,
							Value: v,
							Err:   err,
							Kind:  f.Kind(),
						})
						continue
					}
					iv := reflect.New(f.Type().Elem()).Elem()
					iv.SetInt(intValue)
					f.Set(reflect.Append(f, iv))
				}
			case reflect.Bool:
				for _, v := range vals {
					boolValue, err := strconv.ParseBool(v)
					if err != nil {
						errs = append(errs, &ParseError{
							Field: tp.Field(i).Name,
							Name:  name,
							Value: v,
							Err:   err,
							Kind:  f.Kind(),
						})
						continue
					}
					f.Set(reflect.Append(f, reflect.ValueOf(boolValue)))
				}
			case reflect.Float32, reflect.Float64:
				for _, v := range vals {
					floatValue, err := strconv.ParseFloat(v, f.Type().Elem().Bits())
					if err != nil {
						errs = append(errs, &ParseError{
							Field: tp.Field(i).Name,
							Name:  name,
							Value: v,
							Err:   err,
							Kind:  f.Kind(),
						})
						continue
					}
					fv := reflect.New(f.Type().Elem()).Elem()
					fv.SetFloat(floatValue)
					f.Set(reflect.Append(f, fv))
				}
			default:
				return fmt.Errorf("field %q: unsuported type %s", tp.Field(i).Name, f.Kind())
			}
		default:
			return fmt.Errorf("field %q: unsuported type %s", tp.Field(i).Name, f.Kind())
		}

	}

	if len(errs) == 0 {
		return nil
	}
	return errs
}

var errRequired = errors.New("required")

func contains(arr []string, s string) bool {
	for _, el := range arr {
		if el == s {
			return true
		}
	}
	return false
}

var splitList = struct {
	sync.Mutex
	fn func(string) []string
}{
	fn: func(s string) []string {
		return separatorRx.Split(s, -1)
	},
}

var separatorRx = regexp.MustCompile(`\s*,\s*`)

// SeparatorFunc set list separator function. By default, values are expected
// to be separated by ,
func SeparatorFunc(f func(string) []string) {
	splitList.Lock()
	splitList.fn = f
	splitList.Unlock()
}

type ParseErrors []*ParseError

func (e ParseErrors) Error() string {
	switch n := len(e); n {
	case 0:
		return ""
	case 1:
		return e[0].Error()
	default:
		return fmt.Sprintf("%d parse errors", n)
	}
}

type ParseError struct {
	// Destination structure field name
	Field string
	// Value name as provided in raw configuration
	Name string
	// Value as provided in raw configuration
	Value string
	// Parsing error
	Err error
	// Destination structure field kind
	Kind reflect.Kind
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("cannot parse %s: %s", e.Field, e.Err)
}

func convertName(s string) string {
	s = conv1.ReplaceAllStringFunc(s, func(val string) string {
		return val[:1] + "_" + val[1:]
	})
	s = conv2.ReplaceAllStringFunc(s, func(val string) string {
		return val[:1] + "_" + val[1:]
	})
	return strings.ToUpper(s)
}

var (
	conv1 = regexp.MustCompile(`.([A-Z][a-z]+)`)
	conv2 = regexp.MustCompile(`([a-z0-9])([A-Z])`)
)

// Describe returns string description of given structure or error.
//
// Description contains of columns describing each configuration option name,
// type and default value. Each row represents single configuration value.
func Describe(dest interface{}) (string, error) {
	s := reflect.ValueOf(dest)
	if s.Kind() != reflect.Ptr {
		return "", fmt.Errorf("expected pointer to struct, got %T", dest)
	}

	s = s.Elem()
	if s.Kind() != reflect.Struct {
		return "", fmt.Errorf("expected struct, got %T", dest)
	}

	splitList.Lock()
	defer splitList.Unlock()

	var (
		rows   [][]string
		width1 int
		width2 int
	)

	tp := s.Type()
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		if !f.CanSet() {
			continue
		}
		tags := strings.Split(tp.Field(i).Tag.Get("envconf"), ",")

		var name string
		if len(tags) > 0 && tags[0] != "" {
			name = tags[0]
		} else {
			name = convertName(tp.Field(i).Name)
		}

		required := false
		if len(tags) > 1 && contains(tags[1:], "required") {
			required = true
		}

		if len(name) > width1 {
			width1 = len(name)
		}
		kind := f.Kind().String()
		switch f.Kind() {
		case reflect.Struct:
			kind = f.Type().Name()
		case reflect.Slice:
			if f.Type().Elem().Kind() == reflect.Uint8 {
				kind = "bytes"
			} else {
				kind = f.Type().Elem().Name() + " list"
			}
		}
		if len(kind) > width2 {
			width2 = len(kind)
		}

		var extra string
		if !isZero(f) {
			extra = fmt.Sprintf("\"%v\"", f.Interface())
		} else if required {
			extra = "(required)"
		}

		rows = append(rows, []string{name, kind, extra})
	}

	// format collected rows into nice columns
	var b bytes.Buffer
	for _, row := range rows {
		name := row[0] + strings.Repeat(" ", width1-len(row[0]))
		kind := row[1] + strings.Repeat(" ", width2-len(row[1]))
		line := fmt.Sprintf("%s  %s  %s", name, kind, row[2])
		line = strings.TrimSpace(line)
		fmt.Fprintln(&b, line)
	}
	return b.String(), nil
}

// isZero return true if given value represents zero value of it's type
func isZero(f reflect.Value) bool {
	switch f.Kind() {
	case reflect.String:
		return f.String() == ""
	case reflect.Int:
		return f.Int() == 0
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return f.Int() == 0
	case reflect.Bool:
		return f.Bool() == false
	case reflect.Float32, reflect.Float64:
		return f.Float() == 0
	case reflect.Slice:
		return f.Len() == 0
	case reflect.Ptr:
		return f.IsNil()
	default:
		// unknown type can be ignored
		return true
	}
}
