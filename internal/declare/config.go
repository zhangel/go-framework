package declare

import (
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/xhit/go-str2duration/v2"

	"github.com/zhangel/go-framework/internal"
	"github.com/zhangel/go-framework/uri"
)

const (
	FlagOverwrite = "config.set"
)

var (
	FrameworkFlagSet = flag.NewFlagSet("", flag.ExitOnError)
	AllFlags         = map[string]Flag{}
	allFlagsWithNs   = map[string][]Flag{}
	sensitiveMap     = map[string]struct{}{}
	configOnce       sync.Once
)

type MapFlags map[string]string

func (s *MapFlags) String() string {
	return fmt.Sprintf("%v", *s)
}

func (s *MapFlags) Set(value string) error {
	pair := strings.Split(value, "=")
	if len(pair) != 2 || pair[0] == "" {
		return fmt.Errorf("invalid config value type")
	}

	(*s)[pair[0]] = pair[1]
	return nil
}

var overwriteFlags = MapFlags{}

type durationValue time.Duration

func (d *durationValue) Set(s string) error {
	v, err := str2duration.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = durationValue(v)
	return err
}

func (d *durationValue) Get() interface{} { return time.Duration(*d) }

func (d *durationValue) String() string { return (*time.Duration)(d).String() }

type Flag struct {
	Name            string
	DefaultValue    interface{}
	Description     string
	Env             string
	UriField        uri.UriFieldType
	UriFieldHandler func(string) string
	Sensitive       bool
	Deprecated      bool

	pluginFlagType   bool
	pluginType       string
	pluginName       string
	pluginDeprecated bool
}

func Flags(ns string, flags ...Flag) {
	allFlagsWithNs[ns] = append(allFlagsWithNs[ns], flags...)
}

type ModifiableConfigSource interface {
	Get(k string) (interface{}, bool)
	Put(k string, v interface{})
}

func PopulateAllFlags(source ModifiableConfigSource, flagsToShow, flagsToHide []string) {
	configOnce.Do(func() {
		if source == nil && FrameworkFlagSet.Parsed() {
			log.Fatal("[ERROR] Flag parsed before framework initialized. Remove flag.Parse() from application codes.")
		}

		reg := regexp.MustCompile("[^_A-Z0-9]+")
		envVarName := func(flagName, envName string) string {
			if flagName == "verbose" {
				return ""
			}

			if envName != "" {
				return reg.ReplaceAllString(strings.Replace(strings.ToUpper(envName), ".", "_", -1), "")
			} else {
				return reg.ReplaceAllString(strings.Replace(strings.ToUpper(flagName), ".", "_", -1), "")
			}
		}

		for ns, flags := range allFlagsWithNs {
			for _, f := range flags {
				f.Name = flagName(ns, f.Name)
				f.Env = envVarName(f.Name, f.Env)
				AllFlags[f.Name] = f
			}
		}

		for _, f := range AllFlags {
			if f.DefaultValue == nil {
				log.Fatalf("[ERROR] Default value of %q not declared.\n", f.Name)
			}

			setDefaultValue(source, f.Name, f.DefaultValue)
			if f.Sensitive {
				sensitiveMap[f.Name] = struct{}{}
			}

			if f.Deprecated {
				f.Description = "[DEPRECATED!] " + f.Description
			}

			switch v := f.DefaultValue.(type) {
			case string:
				FrameworkFlagSet.String(f.Name, v, f.Description)
			case bool:
				FrameworkFlagSet.Bool(f.Name, v, f.Description)
			case int:
				FrameworkFlagSet.Int(f.Name, v, f.Description)
			case int8:
				FrameworkFlagSet.Int(f.Name, int(v), f.Description)
			case int16:
				FrameworkFlagSet.Int(f.Name, int(v), f.Description)
			case int32:
				FrameworkFlagSet.Int(f.Name, int(v), f.Description)
			case int64:
				FrameworkFlagSet.Int64(f.Name, v, f.Description)
			case uint:
				FrameworkFlagSet.Uint(f.Name, v, f.Description)
			case uint8:
				FrameworkFlagSet.Uint(f.Name, uint(v), f.Description)
			case uint16:
				FrameworkFlagSet.Uint(f.Name, uint(v), f.Description)
			case uint32:
				FrameworkFlagSet.Uint(f.Name, uint(v), f.Description)
			case uint64:
				FrameworkFlagSet.Uint64(f.Name, v, f.Description)
			case float32:
				FrameworkFlagSet.Float64(f.Name, float64(v), f.Description)
			case float64:
				FrameworkFlagSet.Float64(f.Name, v, f.Description)
			case time.Duration:
				durationVal := durationValue(v)
				FrameworkFlagSet.Var(&durationVal, f.Name, f.Description)
			case MapFlags:
				FrameworkFlagSet.Var(&overwriteFlags, f.Name, f.Description)
			default:
				FrameworkFlagSet.String(f.Name, "", f.Description)
			}
		}

		if source == nil {
			FrameworkFlagSet.Usage = func() { ShowUsage(flagsToShow, flagsToHide, false) }

			flag.VisitAll(func(flag *flag.Flag) {
				flagName := strings.ToLower(flag.Name)
				FrameworkFlagSet.Var(flag.Value, flagName, flag.Usage)
				AllFlags[flag.Name] = Flag{Name: flagName, DefaultValue: flag.Value, Env: envVarName(flagName, "")}
			})

			_ = FrameworkFlagSet.Parse(os.Args[1:])
		}
	})
}

func setDefaultValue(source ModifiableConfigSource, flagName string, defaultVal interface{}) {
	if source == nil {
		return
	}

	if _, ok := source.Get(flagName); ok {
		return
	}

	source.Put(flagName, internal.Stringify(defaultVal))
}

func IsSensitive(flagName string) bool {
	_, ok := sensitiveMap[flagName]
	return ok
}
