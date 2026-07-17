package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"unicode"

	"github.com/ourdatateam/forgejo-cli/internal/cmd"
	"github.com/ourdatateam/forgejo-cli/internal/cmdutil"
)

func main() {
	outDir := flag.String("out", "skills/forgejo-cli/references", "directory to write generated markdown reference files")
	flag.Parse()

	if err := run(*outDir); err != nil {
		fmt.Fprintf(os.Stderr, "gendocs: %v\n", err)
		os.Exit(1)
	}
}

func run(outDir string) error {
	ctx := &cmdutil.Ctx{
		Out: io.Discard,
		Err: io.Discard,
		In:  strings.NewReader(""),
	}
	root := commandValue{v: reflect.ValueOf(cmd.NewRoot(ctx))}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	globalFlags := collectFlags(root.persistentFlags())
	groups := visibleCommands(root.commands())
	for _, group := range groups {
		var buf bytes.Buffer
		writeGroup(&buf, group, globalFlags)

		path := filepath.Join(outDir, safeFileName(group.name())+".md")
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func writeGroup(w *bytes.Buffer, group commandValue, globalFlags []flagDoc) {
	fmt.Fprintf(w, "# forgejo %s\n\n", group.name())
	writeText(w, commandDescription(group))

	fmt.Fprintln(w, "## Global Flags")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "These inherited flags apply to commands in this group unless a command defines a local flag with the same name.")
	fmt.Fprintln(w)
	writeFlagsTable(w, globalFlags)

	commands := descendantCommands(group)
	if len(commands) == 0 {
		writeCommand(w, group, group.root(), false)
		return
	}
	for _, c := range commands {
		writeCommand(w, c, group.root(), true)
	}
}

func writeCommand(w *bytes.Buffer, c commandValue, root commandValue, includeDescription bool) {
	fmt.Fprintf(w, "## %s\n\n", c.commandPath())
	fmt.Fprintf(w, "Use: `%s`\n\n", useLine(c))
	if includeDescription {
		writeText(w, commandDescription(c))
	}
	writeFlagsTable(w, commandFlags(c, root))
}

func commandDescription(c commandValue) string {
	if s := strings.TrimSpace(c.long()); s != "" {
		return s
	}
	return strings.TrimSpace(c.short())
}

func writeText(w *bytes.Buffer, s string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return
	}
	fmt.Fprintln(w, s)
	fmt.Fprintln(w)
}

func useLine(c commandValue) string {
	use := strings.TrimSpace(c.use())
	if use == "" {
		return c.commandPath()
	}
	fields := strings.Fields(use)
	if len(fields) == 0 {
		return c.commandPath()
	}
	args := strings.TrimSpace(strings.TrimPrefix(use, fields[0]))
	if args == "" {
		return c.commandPath()
	}
	return c.commandPath() + " " + args
}

func descendantCommands(group commandValue) []commandValue {
	var out []commandValue
	var walk func(commandValue)
	walk = func(parent commandValue) {
		for _, child := range visibleCommands(parent.commands()) {
			out = append(out, child)
			walk(child)
		}
	}
	walk(group)
	return out
}

func visibleCommands(commands []commandValue) []commandValue {
	out := make([]commandValue, 0, len(commands))
	for _, c := range commands {
		if c.isNil() || c.hidden() {
			continue
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].name() < out[j].name()
	})
	return out
}

func commandFlags(c commandValue, root commandValue) []flagDoc {
	docs := collectFlags(c.localFlags())
	if inherited := collectNonRootInheritedFlags(c, root); len(inherited) > 0 {
		docs = append(docs, inherited...)
		sort.Slice(docs, func(i, j int) bool {
			return docs[i].Name < docs[j].Name
		})
	}
	return docs
}

func collectNonRootInheritedFlags(c commandValue, root commandValue) []flagDoc {
	seen := map[string]bool{}
	visitFlags(root.persistentFlags(), func(f reflect.Value) {
		seen[flagFieldString(f, "Name")] = true
	})

	var docs []flagDoc
	for p, ok := c.parent(); ok && !p.same(root); p, ok = p.parent() {
		visitFlags(p.persistentFlags(), func(f reflect.Value) {
			name := flagFieldString(f, "Name")
			if !flagFieldBool(f, "Hidden") && !seen[name] {
				docs = append(docs, flagFromPFlag(f))
				seen[name] = true
			}
		})
	}
	sort.Slice(docs, func(i, j int) bool {
		return docs[i].Name < docs[j].Name
	})
	return docs
}

type commandValue struct {
	v reflect.Value
}

func (c commandValue) isNil() bool {
	return !c.v.IsValid() || (c.v.Kind() == reflect.Pointer && c.v.IsNil())
}

func (c commandValue) same(other commandValue) bool {
	if c.isNil() || other.isNil() || c.v.Kind() != reflect.Pointer || other.v.Kind() != reflect.Pointer {
		return false
	}
	return c.v.Pointer() == other.v.Pointer()
}

func (c commandValue) name() string        { return c.callString("Name") }
func (c commandValue) commandPath() string { return c.callString("CommandPath") }
func (c commandValue) use() string         { return c.fieldString("Use") }
func (c commandValue) short() string       { return c.fieldString("Short") }
func (c commandValue) long() string        { return c.fieldString("Long") }
func (c commandValue) hidden() bool        { return c.fieldBool("Hidden") }

func (c commandValue) commands() []commandValue {
	if c.isNil() {
		return nil
	}
	values := c.v.MethodByName("Commands").Call(nil)
	if len(values) == 0 || values[0].IsNil() {
		return nil
	}
	commands := values[0]
	out := make([]commandValue, 0, commands.Len())
	for i := 0; i < commands.Len(); i++ {
		out = append(out, commandValue{v: commands.Index(i)})
	}
	return out
}

func (c commandValue) parent() (commandValue, bool) {
	if c.isNil() {
		return commandValue{}, false
	}
	values := c.v.MethodByName("Parent").Call(nil)
	if len(values) == 0 || values[0].IsNil() {
		return commandValue{}, false
	}
	return commandValue{v: values[0]}, true
}

func (c commandValue) root() commandValue {
	if c.isNil() {
		return commandValue{}
	}
	values := c.v.MethodByName("Root").Call(nil)
	if len(values) == 0 || values[0].IsNil() {
		return commandValue{}
	}
	return commandValue{v: values[0]}
}

func (c commandValue) localFlags() reflect.Value {
	return c.callValue("LocalFlags")
}

func (c commandValue) persistentFlags() reflect.Value {
	return c.callValue("PersistentFlags")
}

func (c commandValue) callString(name string) string {
	values := c.call(name)
	if len(values) == 0 {
		return ""
	}
	return values[0].String()
}

func (c commandValue) callValue(name string) reflect.Value {
	values := c.call(name)
	if len(values) == 0 {
		return reflect.Value{}
	}
	return values[0]
}

func (c commandValue) call(name string) []reflect.Value {
	if c.isNil() {
		return nil
	}
	method := c.v.MethodByName(name)
	if !method.IsValid() {
		return nil
	}
	return method.Call(nil)
}

func (c commandValue) fieldString(name string) string {
	field := c.field(name)
	if !field.IsValid() {
		return ""
	}
	return field.String()
}

func (c commandValue) fieldBool(name string) bool {
	field := c.field(name)
	return field.IsValid() && field.Bool()
}

func (c commandValue) field(name string) reflect.Value {
	if c.isNil() {
		return reflect.Value{}
	}
	elem := c.v
	if elem.Kind() == reflect.Pointer {
		elem = elem.Elem()
	}
	return elem.FieldByName(name)
}

type flagDoc struct {
	Name    string
	Type    string
	Default string
	Help    string
}

func collectFlags(flags reflect.Value) []flagDoc {
	var docs []flagDoc
	visitFlags(flags, func(f reflect.Value) {
		if flagFieldBool(f, "Hidden") {
			return
		}
		docs = append(docs, flagFromPFlag(f))
	})
	sort.Slice(docs, func(i, j int) bool {
		return docs[i].Name < docs[j].Name
	})
	return docs
}

func visitFlags(flags reflect.Value, visit func(reflect.Value)) {
	if !flags.IsValid() || (flags.Kind() == reflect.Pointer && flags.IsNil()) {
		return
	}
	method := flags.MethodByName("VisitAll")
	if !method.IsValid() {
		return
	}
	callbackType := method.Type().In(0)
	callback := reflect.MakeFunc(callbackType, func(args []reflect.Value) []reflect.Value {
		if len(args) > 0 {
			visit(args[0])
		}
		return nil
	})
	method.Call([]reflect.Value{callback})
}

func flagFromPFlag(f reflect.Value) flagDoc {
	name := "--" + flagFieldString(f, "Name")
	if shorthand := flagFieldString(f, "Shorthand"); shorthand != "" {
		name = "-" + shorthand + ", " + name
	}
	def := flagFieldString(f, "DefValue")
	if def == "" {
		def = `""`
	}
	return flagDoc{
		Name:    name,
		Type:    flagValueType(f),
		Default: def,
		Help:    flagFieldString(f, "Usage"),
	}
}

func flagFieldString(f reflect.Value, name string) string {
	field := flagField(f, name)
	if !field.IsValid() {
		return ""
	}
	return field.String()
}

func flagFieldBool(f reflect.Value, name string) bool {
	field := flagField(f, name)
	return field.IsValid() && field.Bool()
}

func flagField(f reflect.Value, name string) reflect.Value {
	if !f.IsValid() {
		return reflect.Value{}
	}
	if f.Kind() == reflect.Pointer {
		if f.IsNil() {
			return reflect.Value{}
		}
		f = f.Elem()
	}
	return f.FieldByName(name)
}

func flagValueType(f reflect.Value) string {
	value := flagField(f, "Value")
	if !value.IsValid() || value.IsNil() {
		return ""
	}
	method := value.MethodByName("Type")
	if !method.IsValid() {
		return ""
	}
	values := method.Call(nil)
	if len(values) == 0 {
		return ""
	}
	return values[0].String()
}

func writeFlagsTable(w *bytes.Buffer, flags []flagDoc) {
	fmt.Fprintln(w, "| Name | Type | Default | Help |")
	fmt.Fprintln(w, "| :--- | :--- | :--- | :--- |")
	if len(flags) == 0 {
		fmt.Fprintln(w, "| _None_ |  |  |  |")
		fmt.Fprintln(w)
		return
	}
	for _, f := range flags {
		fmt.Fprintf(w, "| `%s` | `%s` | `%s` | %s |\n",
			escapeCode(f.Name),
			escapeCode(f.Type),
			escapeCode(f.Default),
			escapeTable(f.Help),
		)
	}
	fmt.Fprintln(w)
}

func escapeTable(s string) string {
	s = strings.ReplaceAll(s, "\n", "<br>")
	s = strings.ReplaceAll(s, "|", `\|`)
	return s
}

func escapeCode(s string) string {
	return strings.ReplaceAll(s, "`", "\\`")
}

func safeFileName(s string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case unicode.IsSpace(r), r == '-', r == '_':
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
