package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/ICan-TC/lib/logging"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	loaded = false
)
var config interface{}

func BindConfigStruct(v *viper.Viper, s interface{}, prefix string) {
	t := reflect.TypeOf(s)
	vStruct := reflect.ValueOf(s)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
		vStruct = vStruct.Elem()
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		fieldName := f.Name
		flagTag := f.Tag.Get("flag")
		envTag := f.Tag.Get("env")
		yamlTag := f.Tag.Get("yaml")
		defaultTag := f.Tag.Get("default")

		// Compose key with prefix for nested structs
		key := yamlTag
		if prefix != "" {
			key = prefix + "." + yamlTag
		}

		l := logging.L()
		// Register flag
		if flagTag != "" {
			l.Debug().Msgf("Registering flag for field '%s': flag=%s, env=%s, yaml=%s, default=%s", fieldName, flagTag, envTag, yamlTag, defaultTag)

			switch f.Type.Kind() {
			case reflect.Int, reflect.Int64:
				defVal := 0
				if defaultTag != "" {
					if v, err := strconv.Atoi(defaultTag); err == nil {
						defVal = v
					}
				}
				pflag.Int(flagTag, defVal, fmt.Sprintf("%s (default: %s)", fieldName, defaultTag))
			case reflect.String:
				defVal := ""
				if defaultTag != "" {
					defVal = defaultTag
				}
				pflag.String(flagTag, defVal, fmt.Sprintf("%s (default: %s)", fieldName, defaultTag))
			case reflect.Bool:
				defVal := false
				if defaultTag != "" {
					defVal = defaultTag == "true"
				}
				pflag.Bool(flagTag, defVal, fmt.Sprintf("%s (default: %s)", fieldName, defaultTag))
			case reflect.Float64:
				defVal := 0.0
				if defaultTag != "" {
					if v, err := strconv.ParseFloat(defaultTag, 64); err == nil {
						defVal = v
					}
				}
				pflag.Float64(flagTag, defVal, fmt.Sprintf("%s (default: %s)", fieldName, defaultTag))
			default:
				fmt.Fprintf(os.Stderr, "Unsupported flag type for %s: %s\n", fieldName, f.Type.Kind())
			}
		}
		// Set default
		if defaultTag != "" {
			l.Debug().Msgf("Setting default for key '%s': %s", key, defaultTag)
			v.SetDefault(key, defaultTag)
		}
		// Bind env
		if envTag != "" {
			v.BindEnv(key, envTag)
		}
	}
}

func ValidateConfigStruct(s interface{}) error {
	t := reflect.TypeOf(s)
	vStruct := reflect.ValueOf(s)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
		vStruct = vStruct.Elem()
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		val := vStruct.Field(i)
		// Recursively validate nested structs
		if f.Type.Kind() == reflect.Struct {
			if err := ValidateConfigStruct(val.Interface()); err != nil {
				return fmt.Errorf("%s: %w", f.Name, err)
			}
			continue
		}
		validateTag := f.Tag.Get("validate")
		if validateTag == "" {
			continue
		}
		for _, rule := range strings.Split(validateTag, ",") {
			rule = strings.TrimSpace(rule)
			if rule == "required" && IsZero(val) {
				return fmt.Errorf("%s is required", f.Name)
			}
			if after, ok := strings.CutPrefix(rule, "min="); ok {
				min := Atoi(after)
				if val.Int() < int64(min) {
					return fmt.Errorf("%s must be >= %d", f.Name, min)
				}
			}
			if after, ok := strings.CutPrefix(rule, "max="); ok {
				max := Atoi(after)
				if val.Int() > int64(max) {
					return fmt.Errorf("%s must be <= %d", f.Name, max)
				}
			}
			if after, ok := strings.CutPrefix(rule, "oneof="); ok {
				opts := strings.Split(after, " ")
				found := false
				for _, opt := range opts {
					if val.String() == opt {
						found = true
					}
				}
				if !found {
					return fmt.Errorf("%s must be one of %v", f.Name, opts)
				}
			}
		}
	}
	return nil
}

func IsZero(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Int, reflect.Int64:
		return v.Int() == 0
	}
	return false
}

func Atoi(s string) int {
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}
