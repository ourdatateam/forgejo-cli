package cmd

import (
	"strings"

	"github.com/ourdatateam/forgejo-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

type repoFlagMatch struct {
	index int
	value string
	width int
	long  bool
}

// PreprocessRepoFlag rewrites gh-style -R/--repo into the repo positional
// expected by the existing command grammar.
func PreprocessRepoFlag(root *cobra.Command, argv []string) ([]string, error) {
	cmd, _ := walkCommandPath(root, argv)
	if commandDisablesFlagParsing(cmd) {
		return argv, nil
	}
	long, ok := firstRepoFlagKind(argv)
	if !ok {
		return argv, nil
	}
	if long && commandHasLocalRepoFlag(cmd) {
		return argv, nil
	}
	match, ok, err := findRepoFlag(argv)
	if err != nil {
		return nil, err
	}

	stripped := removeRepoFlag(argv, match)
	cmd, insertAt := walkCommandPath(root, stripped)
	if commandDisablesFlagParsing(cmd) {
		return argv, nil
	}
	if match.long && commandHasLocalRepoFlag(cmd) {
		return argv, nil
	}
	if match.value != "." && !cmdutil.ValidRepo(match.value) {
		return nil, cmdutil.Usagef("invalid repo %q (expected owner/repo, or '.' to use the current directory's remote)", match.value)
	}

	out := make([]string, 0, len(stripped)+1)
	out = append(out, stripped[:insertAt]...)
	out = append(out, match.value)
	out = append(out, stripped[insertAt:]...)
	return out, nil
}

func firstRepoFlagKind(argv []string) (bool, bool) {
	for _, arg := range argv {
		if arg == "--" {
			return false, false
		}
		switch {
		case arg == "-R" || strings.HasPrefix(arg, "-R="):
			return false, true
		case arg == "--repo" || strings.HasPrefix(arg, "--repo="):
			return true, true
		}
	}
	return false, false
}

func findRepoFlag(argv []string) (repoFlagMatch, bool, error) {
	for i, arg := range argv {
		if arg == "--" {
			return repoFlagMatch{}, false, nil
		}
		switch {
		case arg == "-R" || arg == "--repo":
			if i+1 >= len(argv) {
				return repoFlagMatch{}, false, cmdutil.Usagef("%s requires a value", arg)
			}
			return repoFlagMatch{index: i, value: argv[i+1], width: 2, long: arg == "--repo"}, true, nil
		case strings.HasPrefix(arg, "-R="):
			return repoFlagMatch{index: i, value: strings.TrimPrefix(arg, "-R="), width: 1}, true, nil
		case strings.HasPrefix(arg, "--repo="):
			return repoFlagMatch{index: i, value: strings.TrimPrefix(arg, "--repo="), width: 1, long: true}, true, nil
		}
	}
	return repoFlagMatch{}, false, nil
}

func removeRepoFlag(argv []string, match repoFlagMatch) []string {
	out := make([]string, 0, len(argv)-match.width)
	out = append(out, argv[:match.index]...)
	out = append(out, argv[match.index+match.width:]...)
	return out
}

func walkCommandPath(root *cobra.Command, argv []string) (*cobra.Command, int) {
	cmd := root
	insertAt := 0
	for i := 0; i < len(argv); {
		arg := argv[i]
		if arg == "--" {
			break
		}
		if skip := flagArgWidth(cmd, arg); skip > 0 {
			i += skip
			continue
		}
		child := matchingChild(cmd, arg)
		if child == nil {
			break
		}
		cmd = child
		i++
		insertAt = i
	}
	return cmd, insertAt
}

func flagArgWidth(cmd *cobra.Command, arg string) int {
	if len(arg) < 2 || arg[0] != '-' || arg == "-" {
		return 0
	}
	if strings.HasPrefix(arg, "--") {
		name := strings.TrimPrefix(arg, "--")
		hasValue := false
		if name == "" {
			return 0
		}
		if before, _, ok := strings.Cut(name, "="); ok {
			name = before
			hasValue = true
		}
		flag := cmd.Flag(name)
		if flag == nil {
			return 0
		}
		if hasValue || flag.NoOptDefVal != "" {
			return 1
		}
		return 2
	}
	if strings.HasPrefix(arg, "-R=") {
		if cmd.Flag("repo") != nil {
			return 1
		}
		return 0
	}
	if arg == "-R" {
		if flag := cmd.Flag("repo"); flag != nil {
			if flag.NoOptDefVal != "" {
				return 1
			}
			return 2
		}
	}
	return 0
}

func matchingChild(cmd *cobra.Command, arg string) *cobra.Command {
	for _, child := range cmd.Commands() {
		if child.Name() == arg {
			return child
		}
		for _, alias := range child.Aliases {
			if alias == arg {
				return child
			}
		}
	}
	return nil
}

func commandDisablesFlagParsing(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c.DisableFlagParsing {
			return true
		}
	}
	return false
}

func commandHasLocalRepoFlag(cmd *cobra.Command) bool {
	return cmd != nil && cmd.LocalFlags().Lookup("repo") != nil
}
