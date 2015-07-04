package optigo

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

type actionType int

const (
	atINCREMENT actionType = iota
	atAPPEND
	atASSIGN
)

type dataType int

const (
	dtSTRING dataType = iota
	dtINTEGER
	dtFLOAT
	dtBOOLEAN
)

type option struct {
	name     string
	unary    bool
	dest     reflect.Value
	action   actionType
	dataType dataType
}

func (o *option) parseValue(val string) (interface{}, error) {
	switch o.dataType {
	case dtSTRING:
		return val, nil
	case dtINTEGER:
		if i, err := strconv.ParseInt(val, 10, 64); err == nil {
			return i, nil
		} else {
			return nil, err
		}
	case dtFLOAT:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f, nil
		} else {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("Unable to parse value: %s", val)
	}
}

type actions map[string]option

func parseAction(spec string, dest interface{}, actions map[string]option) error {
	unary := false
	var a actionType
	var t dataType
	if spec[len(spec)-1] == '+' {
		unary = true
		a = atINCREMENT
		spec = spec[0 : len(spec)-1]
	} else if spec[len(spec)-1] == '@' {
		a = atAPPEND
		spec = spec[0 : len(spec)-1]
	} else {
		a = atASSIGN
	}

	switch spec[len(spec)-2:] {
	case "=s":
		t = dtSTRING
		spec = spec[0 : len(spec)-2]
	case "=i":
		t = dtINTEGER
		spec = spec[0 : len(spec)-2]
	case "=f":
		t = dtFLOAT
		spec = spec[0 : len(spec)-2]
	default:
		if a == atINCREMENT {
			t = dtINTEGER
		} else {
			t = dtBOOLEAN
		}
		unary = true
	}

	if unary && a == atAPPEND {
		return fmt.Errorf("invalid spec, using @ to parse repeated options, but not specifying type with either =i =s or =f: %s", spec)
	}

	optionNames := strings.Split(spec, "|")
	name := optionNames[len(optionNames)-1]
	for _, opt := range optionNames {
		var dashName string
		if len(opt) == 1 {
			dashName = "-" + opt
		} else {
			dashName = "--" + opt
		}
		if _, ok := actions[dashName]; ok {
			return fmt.Errorf("invalid option spec: %s is not unique from %s", dashName, spec)
		}
		actions[dashName] = option{name, unary, reflect.ValueOf(dest), a, t}
	}
	return nil
}

func increment(val reflect.Value) reflect.Value {
	return reflect.ValueOf(val.Int() + 1)
}

func push(arr reflect.Value, val interface{}) reflect.Value {
	rVal := reflect.ValueOf(val)
	if rVal.Type() != arr.Type().Elem() {
		// The value type is not the same as the array value type
		// so try to convert the passed in value to the array value type
		newValPtr := reflect.New(arr.Type().Elem())
		newValPtr.Elem().Set(rVal.Convert(arr.Type().Elem()))
		rVal = newValPtr.Elem()
	}
	return reflect.Append(arr, rVal)
}

func initResultMap(actions actions) map[string]interface{} {
	results := make(map[string]interface{})
	for _, opt := range actions {
		if opt.unary {
			if opt.action == atINCREMENT {
				results[opt.name] = int64(0)
			} else {
				results[opt.name] = false
			}
		} else {
			if opt.action == atAPPEND {
				switch opt.dataType {
				case dtSTRING:
					results[opt.name] = make([]string, 0)
				case dtINTEGER:
					results[opt.name] = make([]int64, 0)
				case dtFLOAT:
					results[opt.name] = make([]float64, 0)
				}
			} else {
				switch opt.dataType {
				case dtSTRING:
					results[opt.name] = ""
				case dtINTEGER:
					results[opt.name] = int64(0)
				case dtFLOAT:
					results[opt.name] = float64(0)
				}
			}
		}
	}
	return results
}

// OptionParser struct will contain the `Results` and `Args` after
// one of the Process routines is called.  A OptionParser object
// is created with either NewParser or NewDirectAssignParser
type OptionParser struct {
	actions actions
	Results map[string]interface{}
	Args    []string
}

// NewParser generates an OptionParser object from the opts passed in.
// After calling OptionParser.Parser([]string) the option results will
// be stored in OptionParser.Results
func NewParser(opts []string) OptionParser {
	actions := make(actions)
	for _, spec := range opts {
		if err := parseAction(spec, nil, actions); err != nil {
			panic(err)
		}
	}
	results := initResultMap(actions)
	return OptionParser{actions, results, nil}
}

// NewDirectAssignParser generates an OptionParser object from the `opts` passed in.
// After calling OptionParser.Parser([]string) the options will be assigned directly
// to the references passed in `opts`.
func NewDirectAssignParser(opts map[string]interface{}) OptionParser {
	actions := make(actions)
	for spec, ref := range opts {
		if err := parseAction(spec, ref, actions); err != nil {
			panic(err)
		}
	}
	return OptionParser{actions, nil, nil}
}

// ProcessAll will parse all arguments in args.  If there are any
// arguments in args that start with '-' and are not known
// options then an error will be returned.  Any non-options will
// be available in OptionParser.Args.
func (o *OptionParser) ProcessAll(args []string) error {
	err := o.ProcessSome(args)
	if err != nil {
		return err
	} else {
		for _, opt := range o.Args {
			if opt[0] == '-' {
				return fmt.Errorf("Unknown option: %s", opt)
			}
		}
	}
	return nil
}

// ProcessSome will parse all known arguments in args.  Any non-options
// and unknown options will be available in OPtionParser.Args.  This
// can be used to implement multple pass options parsing, for example
// perhaps sub-commands options are parsed seperately from global options.
func (o *OptionParser) ProcessSome(args []string) error {
	o.Args = make([]string, 0)
	for len(args) > 0 {
		if args[0] == "--" {
			o.Args = append(o.Args, args[1:]...)
			return nil
		}
		if opt, ok := o.actions[args[0]]; ok {
			var value interface{}
			var err error
			if opt.unary {
				value = true
				args = args[1:]
			} else {
				if len(args) < 2 {
					return fmt.Errorf("missing argument value for option: --%s", opt.name)
				} else {
					if value, err = opt.parseValue(args[1]); err != nil {
						return err
					}
				}
				args = args[2:]
			}

			if opt.dest.IsValid() {
				switch opt.action {
				case atINCREMENT:
					opt.dest.Elem().Set(increment(opt.dest.Elem()))
				case atAPPEND:
					opt.dest.Elem().Set(push(opt.dest.Elem(), value))
				case atASSIGN:
					if opt.dest.Kind() == reflect.Func {
						t := reflect.TypeOf(opt.dest.Interface())
						var cbArgs []reflect.Value
						if t.NumIn() == 1 {
							cbArgs = make([]reflect.Value, 1)
							cbArgs[0] = reflect.ValueOf(value)
						} else if t.NumIn() == 2 {
							cbArgs = make([]reflect.Value, 2)
							cbArgs[0] = reflect.ValueOf(opt.name)
							cbArgs[1] = reflect.ValueOf(value)
						}
						opt.dest.Call(cbArgs)
					} else {
						opt.dest.Elem().Set(reflect.ValueOf(value))
					}
				}
			} else {
				switch opt.action {
				case atINCREMENT:
					o.Results[opt.name] = increment(reflect.ValueOf(o.Results[opt.name])).Interface()
				case atAPPEND:
					o.Results[opt.name] = push(reflect.ValueOf(o.Results[opt.name]), value).Interface()
				case atASSIGN:
					o.Results[opt.name] = reflect.ValueOf(value).Interface()
				}
			}
		} else {
			o.Args = append(o.Args, args[0])
			args = args[1:]
		}
	}
	return nil
}
