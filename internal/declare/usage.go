package declare

import (
	"flag"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

type pluginFlagInfo struct {
	pluginType       string
	pluginName       string
	pluginDeprecated bool
}

type nameSpaceStrings []string

func (p nameSpaceStrings) Len() int { return len(p) }
func (p nameSpaceStrings) Less(i, j int) bool {
	is := strings.Split(p[i], ".")
	js := strings.Split(p[j], ".")

	if len(is) <= 2 || len(js) <= 2 {
		if len(is) == len(js) {
			return p[i] < p[j]
		} else {
			return len(is) < len(js)
		}
	}

	return p[i] < p[j]
}

func (p nameSpaceStrings) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

func sortNameSpaceStrings(a []string) { sort.Sort(nameSpaceStrings(a)) }

func ShowUsage(flagsToShow, flagsToHide []string, includeDeprecated bool) {
	_, _ = fmt.Fprintf(os.Stderr, "Usage of %s:\n\n", os.Args[0])

	var pluginSelector []string
	var customFlags []string
	globalFlags := map[string][]string{}
	pluginFlags := map[pluginFlagInfo][]string{}
	declaredFlags := map[string]struct{}{}

	for ns, flags := range allFlagsWithNs {
		for _, f := range flags {
			flagName := flagName(ns, f.Name)
			declaredFlags[flagName] = struct{}{}

			if (f.Deprecated && !includeDeprecated) || !isFlagShow(flagName, flagsToShow, flagsToHide) {
				continue
			}

			if f.pluginTypeFlag {
				pluginSelector = append(pluginSelector, flagName)
			} else if f.pluginType != "" {
				pi := pluginFlagInfo{f.pluginType, f.pluginName, f.pluginDeprecated}
				pluginFlags[pi] = append(pluginFlags[pi], flagName)
			} else if ns != "" {
				globalFlags[ns] = append(globalFlags[ns], flagName)
			} else {
				customFlags = append(customFlags, flagName)
			}
		}
	}

	FrameworkFlagSet.VisitAll(func(flag *flag.Flag) {
		if _, ok := declaredFlags[flag.Name]; !ok {
			customFlags = append(customFlags, flag.Name)
		}
	})

	printCustomFlags(customFlags)
	printPluginSelector(pluginSelector)
	printGlobalFlags(globalFlags)
	printPluginFlags(pluginFlags)
}

func isFlagShow(flagName string, flagsToShow, flagsToHide []string) bool {
	if len(flagsToShow) == 0 && len(flagsToHide) == 0 {
		return true
	} else if len(flagsToShow) > 0 {
		for _, f := range flagsToShow {
			if matched, err := regexp.MatchString(f, flagName); err != nil || matched {
				return true
			}
		}
		return false
	} else {
		for _, f := range flagsToHide {
			if matched, err := regexp.MatchString(f, flagName); err != nil || matched {
				return false
			}
		}
		return true
	}
}

func printCustomFlags(flags []string) {
	if len(flags) == 0 {
		return
	}

	_, _ = fmt.Fprint(os.Stderr, "Options:\n")
	sortNameSpaceStrings(flags)

	for _, name := range flags {
		f := FrameworkFlagSet.Lookup(name)
		if f == nil {
			continue
		}

		printFlag(f)
	}
	_, _ = fmt.Fprint(os.Stderr, "\n")
}

func printPluginSelector(flags []string) {
	if len(flags) == 0 {
		return
	}

	_, _ = fmt.Fprint(os.Stderr, "Plugin selectors:\n")
	sortNameSpaceStrings(flags)

	for _, name := range flags {
		f := FrameworkFlagSet.Lookup(name)
		if f == nil {
			continue
		}

		printFlag(f)
	}
	_, _ = fmt.Fprint(os.Stderr, "\n")
}

func printGlobalFlags(flags map[string][]string) {
	var ns []string
	for k := range flags {
		ns = append(ns, k)
	}

	sortNameSpaceStrings(ns)
	for _, nsItem := range ns {
		_, _ = fmt.Fprintf(os.Stderr, "%s options:\n", capitalize(nsItem))
		nsFlags := flags[nsItem]
		sortNameSpaceStrings(nsFlags)
		for _, fname := range nsFlags {
			f := FrameworkFlagSet.Lookup(fname)
			if f == nil {
				continue
			}
			printFlag(f)
		}
		_, _ = fmt.Fprint(os.Stderr, "\n")
	}
}

func printPluginFlags(flags map[pluginFlagInfo][]string) {
	var ns []pluginFlagInfo
	for k := range flags {
		ns = append(ns, k)
	}

	sort.Slice(ns, func(l, h int) bool {
		return ns[l].pluginType+ns[l].pluginName < ns[h].pluginType+ns[h].pluginName
	})

	for _, pi := range ns {
		if pi.pluginDeprecated {
			continue
		}

		_, _ = fmt.Fprintf(os.Stderr, "%s %s plugin options:\n", capitalize(pi.pluginName), pi.pluginType)
		nsFlags := flags[pi]
		sortNameSpaceStrings(nsFlags)
		for _, fname := range nsFlags {
			f := FrameworkFlagSet.Lookup(fname)
			if f == nil {
				continue
			}
			printFlag(f)
		}

		_, _ = fmt.Fprint(os.Stderr, "\n")
	}
}

func capitalize(str string) string {
	for i, v := range str {
		return string(unicode.ToUpper(v)) + str[i+1:]
	}
	return ""
}

func flagName(ns, name string) string {
	var flagName string
	if ns != "" && !strings.HasPrefix(name, ns+".") {
		flagName = ns + "." + name
	} else {
		flagName = name
	}
	return strings.ToLower(flagName)
}

func printFlag(f *flag.Flag) {
	envName := AllFlags[f.Name].Env
	var s string
	if envName == "" {
		s = fmt.Sprintf("  -%s", f.Name)
	} else {
		s = fmt.Sprintf("  -%s ($%s)", f.Name, envName)
	}
	name, usage := flag.UnquoteUsage(f)
	if len(name) > 0 {
		s += " " + name
	}

	if len(s) <= 4 {
		s += "\t"
	} else {
		s += "\n    \t"
	}
	s += strings.Replace(usage, "\n", "\n    \t", -1)

	if !isZeroValue(f, f.DefValue) {
		if reflect.TypeOf(f.Value).String() == "*flag.stringValue" {
			s += fmt.Sprintf(" (default %q)", f.DefValue)
		} else {
			s += fmt.Sprintf(" (default %v)", f.DefValue)
		}
	}
	_, _ = fmt.Fprint(os.Stderr, s, "\n")
}

func isZeroValue(f *flag.Flag, value string) bool {
	typ := reflect.TypeOf(f.Value)
	var z reflect.Value
	if typ.Kind() == reflect.Ptr {
		z = reflect.New(typ.Elem())
	} else {
		z = reflect.Zero(typ)
	}

	if typ.Kind() == reflect.Bool || (typ.Kind() == reflect.Ptr && typ.Elem().Kind() == reflect.Bool) {
		return false
	}
	return value == z.Interface().(flag.Value).String()
}
