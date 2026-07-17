package cmd

// Ported from the bash cmd_user family (forgejo:2526-2911). The token
// endpoints deliberately use HTTP Basic auth because Forgejo rejects bearer
// tokens for users/{user}/tokens.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/ourdatateam/forgejo-cli/internal/api"
	"github.com/ourdatateam/forgejo-cli/internal/cmdutil"
	"github.com/ourdatateam/forgejo-cli/internal/config"
	"github.com/spf13/cobra"
)

func init() { Register(newUserCmd) }

func newUserCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user <list|create|view|edit|delete|key|gpg|token>",
		Short: "Manage users, keys, GPG keys, and access tokens",
		Long: "Manage users through the same admin and user endpoints as the bash CLI.\n\n" +
			"User create/delete are admin endpoints. SSH keys can target either --self or another user; GPG add/delete are self-only because Forgejo has no admin-on-behalf endpoint. Access token verbs authenticate with HTTP Basic auth using FORGEJO_PASSWORD from the config.",
	}
	cmd.AddCommand(
		newUserListCmd(ctx),
		newUserCreateCmd(ctx),
		newUserViewCmd(ctx),
		newUserEditCmd(ctx),
		newUserDeleteCmd(ctx),
		newUserKeyCmd(ctx),
		newUserGPGCmd(ctx),
		newUserTokenCmd(ctx),
	)
	return cmd
}

func newUserListCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List users",
		Long:  "List instance users through the admin/users endpoint. The bash command fetched admin/users?limit=50; the global --limit flag overrides that page size, and --limit=0 fetches all pages.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			n := ctx.ListLimit(50)
			lr, err := ctx.Client.DoList("admin/users", n)
			if err != nil {
				return err
			}
			if ctx.WantsJSON() {
				return ctx.EmitJSON(lr.Body)
			}
			items, err := cmdutil.ParseArray(lr.Body)
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(items))
			for _, m := range items {
				rows = append(rows, []string{
					cmdutil.Str(m, "login"),
					cmdutil.Str(m, "email"),
					yesNo(cmdutil.Str(m, "is_admin")),
					yesNo(cmdutil.Str(m, "active")),
					date10(cmdutil.Str(m, "created")),
				})
			}
			ctx.Table([]string{"LOGIN", "EMAIL", "ADMIN", "ACTIVE", "CREATED"}, rows)
			ctx.Trailer(len(items), lr.Total, n)
			return nil
		},
	}
	return cmd
}

func newUserCreateCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create --login=X --email=X --password=X [--fullname=X] [--admin]",
		Short: "Create a user",
		Long: "Create a user through admin/users. --login, --email, and --password are required.\n\n" +
			"Forgejo's create-user option cannot set site admin directly, so --admin first creates the user and then sends the same follow-up PATCH used by user edit --admin=true. If that promotion fails, the user still exists and the command reports the gap.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			login, _ := cmd.Flags().GetString("login")
			email, _ := cmd.Flags().GetString("email")
			password, _ := cmd.Flags().GetString("password")
			fullname, _ := cmd.Flags().GetString("fullname")
			admin, _ := cmd.Flags().GetBool("admin")
			if login == "" {
				return cmdutil.Usagef("Missing --login")
			}
			if email == "" {
				return cmdutil.Usagef("Missing --email")
			}
			if password == "" {
				return cmdutil.Usagef("Missing --password")
			}

			body, err := json.Marshal(map[string]any{
				"username":             login,
				"email":                email,
				"password":             password,
				"full_name":            fullname,
				"must_change_password": true,
				"login_name":           login,
				"source_id":            0,
				"visibility":           "public",
			})
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("POST", "admin/users", body)
			if err != nil {
				return err
			}

			if admin {
				promote, err := json.Marshal(map[string]any{
					"login_name": login,
					"source_id":  0,
					"admin":      true,
				})
				if err != nil {
					return err
				}
				if _, err := ctx.Client.Do("PATCH", "admin/users/"+cmdutil.NameSeg(login), promote); err != nil {
					return fmt.Errorf("User '%s' created, but granting admin failed.\nRetry with: forgejo user edit %s --admin=true", login, login)
				}
			}

			if ctx.WantsJSON() {
				return ctx.EmitJSON(raw)
			}
			obj, err := cmdutil.ParseObject(raw)
			if err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Created user: %s (%s)\n", cmdutil.Str(obj, "login"), cmdutil.Str(obj, "email"))
			if admin {
				fmt.Fprintln(ctx.Out, "Granted site admin.")
			}
			return nil
		},
	}
	cmd.Flags().String("login", "", "username to create (required)")
	cmd.Flags().String("email", "", "email address for the new user (required)")
	cmd.Flags().String("password", "", "initial password for the new user (required)")
	cmd.Flags().String("fullname", "", "full name to store on the user")
	cmd.Flags().Bool("admin", false, "promote the new user to site admin after creation")
	return cmd
}

func newUserViewCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view <username>",
		Short: "View a user",
		Long:  "View a user by username through users/{username}. Text output prints the bash fields: login, full name, email, admin status, and creation date.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			username := args[0]
			raw, err := ctx.Client.Do("GET", "users/"+cmdutil.NameSeg(username), nil)
			if err != nil {
				return err
			}
			if ctx.WantsJSON() {
				return ctx.EmitJSON(raw)
			}
			obj, err := cmdutil.ParseObject(raw)
			if err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Login:    %s\n", cmdutil.Str(obj, "login"))
			fmt.Fprintf(ctx.Out, "Name:     %s\n", strDefault(obj, "full_name", "-"))
			fmt.Fprintf(ctx.Out, "Email:    %s\n", strDefault(obj, "email", "-"))
			fmt.Fprintf(ctx.Out, "Admin:    %s\n", yesNo(cmdutil.Str(obj, "is_admin")))
			fmt.Fprintf(ctx.Out, "Created:  %s\n", date10(cmdutil.Str(obj, "created")))
			return nil
		},
	}
	return cmd
}

func newUserEditCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <username> [--email=X] [--admin=true|false] [--active=true|false]",
		Short: "Edit a user",
		Long: "Edit a user through admin/users/{username}. The command first fetches users/{username} to preserve the required login_name and source_id fields, matching the bash GET-first-then-PATCH flow.\n\n" +
			"--admin and --active accept true or false; any value other than true is sent as false, matching the bash implementation.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			username := args[0]
			currentRaw, err := ctx.Client.Do("GET", "users/"+cmdutil.NameSeg(username), nil)
			if err != nil {
				return err
			}
			current, err := cmdutil.ParseObject(currentRaw)
			if err != nil {
				return err
			}
			bodyMap := map[string]any{
				"login_name": cmdutil.Str(current, "login"),
				"source_id":  numberDefault(current, "source_id", 0),
			}
			if cmd.Flags().Changed("email") {
				email, _ := cmd.Flags().GetString("email")
				bodyMap["email"] = email
			}
			if cmd.Flags().Changed("admin") {
				admin, _ := cmd.Flags().GetString("admin")
				bodyMap["admin"] = bashBool(admin)
			}
			if cmd.Flags().Changed("active") {
				active, _ := cmd.Flags().GetString("active")
				bodyMap["active"] = bashBool(active)
			}
			body, err := json.Marshal(bodyMap)
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("PATCH", "admin/users/"+cmdutil.NameSeg(username), body)
			if err != nil {
				return err
			}
			if ctx.WantsJSON() {
				return ctx.EmitJSON(raw)
			}
			obj, err := cmdutil.ParseObject(raw)
			if err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Updated user: %s\n", cmdutil.Str(obj, "login"))
			return nil
		},
	}
	cmd.Flags().String("email", "", "set the user's email address")
	cmd.Flags().String("admin", "", "set site admin status (true or false)")
	cmd.Flags().String("active", "", "set active status (true or false)")
	return cmd
}

func newUserDeleteCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <username> [--yes]",
		Short: "Delete a user",
		Long:  "Delete a user through admin/users/{username}. This destructive command requires --yes or an interactive typed-name confirmation.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			user := args[0]
			if err := ctx.ConfirmDelete(cmd, "user", user); err != nil {
				return err
			}
			if _, err := ctx.Client.Do("DELETE", "admin/users/"+cmdutil.NameSeg(user), nil); err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Deleted user: %s\n", user)
			return nil
		},
	}
	cmdutil.AddYesFlag(cmd)
	return cmd
}

func newUserKeyCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "key <list|add|delete>",
		Short: "Manage SSH keys",
		Long:  "Manage SSH keys for --self or for another user. For another user, add/delete use admin/users/{user}/keys while list uses the public users/{user}/keys endpoint.",
	}
	cmd.AddCommand(newUserKeyListCmd(ctx), newUserKeyAddCmd(ctx), newUserKeyDeleteCmd(ctx))
	return cmd
}

func newUserKeyListCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <user> | --self",
		Short: "List SSH keys",
		Long:  "List SSH keys for a user or for the authenticated account with --self. The bash command fetched ?limit=50; the global --limit flag overrides that page size, and --limit=0 fetches all pages.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target, self, err := keyTarget(cmd, args, "Usage: forgejo user key list <user|--self>")
			if err != nil {
				return err
			}
			endpoint := "user/keys"
			if !self {
				endpoint = "users/" + cmdutil.NameSeg(target) + "/keys"
			}
			n := ctx.ListLimit(50)
			lr, err := ctx.Client.DoList(endpoint, n)
			if err != nil {
				return err
			}
			if ctx.WantsJSON() {
				return ctx.EmitJSON(lr.Body)
			}
			items, err := cmdutil.ParseArray(lr.Body)
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(items))
			for _, m := range items {
				rows = append(rows, []string{
					cmdutil.Str(m, "id"),
					strDefault(m, "title", "-"),
					strDefault(m, "fingerprint", "-"),
					date10(cmdutil.Str(m, "created_at")),
				})
			}
			ctx.Table([]string{"ID", "TITLE", "FINGERPRINT", "CREATED"}, rows)
			ctx.Trailer(len(items), lr.Total, n)
			return nil
		},
	}
	cmd.Flags().Bool("self", false, "list SSH keys for the authenticated user")
	return cmd
}

func newUserKeyAddCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <user> --title=X --key=<key-or-@file> | --self --title=X --key=<key-or-@file>",
		Short: "Add an SSH key",
		Long:  "Add an SSH key for a user or for the authenticated account with --self. --title and --key are required. --key is a literal key unless it starts with @, in which case the rest is read as a file path and trailing newlines are stripped.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target, self, err := keyTarget(cmd, args, "Usage: forgejo user key add <user|--self> --title=X --key=<key-or-@file>")
			if err != nil {
				return err
			}
			title, _ := cmd.Flags().GetString("title")
			keyArg, _ := cmd.Flags().GetString("key")
			if title == "" {
				return cmdutil.Usagef("Missing --title")
			}
			if keyArg == "" {
				return cmdutil.Usagef("Missing --key (literal or @file)")
			}
			keyContent, err := readKeyArg(keyArg)
			if err != nil {
				return err
			}
			body, err := json.Marshal(map[string]any{
				"title":     title,
				"key":       keyContent,
				"read_only": false,
			})
			if err != nil {
				return err
			}
			endpoint := "user/keys"
			if !self {
				endpoint = "admin/users/" + cmdutil.NameSeg(target) + "/keys"
			}
			raw, err := ctx.Client.Do("POST", endpoint, body)
			if err != nil {
				return err
			}
			if ctx.WantsJSON() {
				return ctx.EmitJSON(raw)
			}
			obj, err := cmdutil.ParseObject(raw)
			if err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Added SSH key: %s (id=%s)\n", cmdutil.Str(obj, "title"), cmdutil.Str(obj, "id"))
			return nil
		},
	}
	cmd.Flags().Bool("self", false, "add the SSH key to the authenticated user")
	cmd.Flags().String("title", "", "title for the SSH key (required)")
	cmd.Flags().String("key", "", "literal SSH public key, or @path to read a key file (required)")
	return cmd
}

func newUserKeyDeleteCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <user> <id> | --self <id> [--yes]",
		Short: "Delete an SSH key",
		Long:  "Delete an SSH key by numeric id. For --self the command uses user/keys/{id}; for another user it uses admin/users/{user}/keys/{id}. This destructive command requires --yes or an interactive typed-name confirmation.",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			target, self, keyID, err := keyTargetID(cmd, args, "Usage: forgejo user key delete <user|--self> <id> [--yes]")
			if err != nil {
				return err
			}
			keyID, err = cmdutil.IDArg(keyID, "SSH key id")
			if err != nil {
				return err
			}
			if err := ctx.ConfirmDelete(cmd, "SSH key", keyID); err != nil {
				return err
			}
			endpoint := "user/keys/" + keyID
			if !self {
				endpoint = "admin/users/" + cmdutil.NameSeg(target) + "/keys/" + keyID
			}
			if _, err := ctx.Client.Do("DELETE", endpoint, nil); err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Deleted SSH key: %s\n", keyID)
			return nil
		},
	}
	cmd.Flags().Bool("self", false, "delete the SSH key from the authenticated user")
	cmdutil.AddYesFlag(cmd)
	return cmd
}

func newUserGPGCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gpg <list|add|delete>",
		Short: "Manage GPG keys",
		Long:  "Manage GPG keys. Listing can target --self or a user, but add/delete are self-only because Forgejo has no admin-on-behalf endpoint for GPG keys.",
	}
	cmd.AddCommand(newUserGPGListCmd(ctx), newUserGPGAddCmd(ctx), newUserGPGDeleteCmd(ctx))
	return cmd
}

func newUserGPGListCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <user> | --self",
		Short: "List GPG keys",
		Long:  "List GPG keys for a user or for the authenticated account with --self. The bash command fetched ?limit=50; the global --limit flag overrides that page size, and --limit=0 fetches all pages.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target, self, err := keyTarget(cmd, args, "Usage: forgejo user gpg list <user|--self>")
			if err != nil {
				return err
			}
			endpoint := "user/gpg_keys"
			if !self {
				endpoint = "users/" + cmdutil.NameSeg(target) + "/gpg_keys"
			}
			n := ctx.ListLimit(50)
			lr, err := ctx.Client.DoList(endpoint, n)
			if err != nil {
				return err
			}
			if ctx.WantsJSON() {
				return ctx.EmitJSON(lr.Body)
			}
			items, err := cmdutil.ParseArray(lr.Body)
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(items))
			for _, m := range items {
				rows = append(rows, []string{
					cmdutil.Str(m, "id"),
					cmdutil.Str(m, "key_id"),
					gpgEmails(m),
					date10(cmdutil.Str(m, "created_at")),
				})
			}
			ctx.Table([]string{"ID", "KEY_ID", "EMAILS", "CREATED"}, rows)
			ctx.Trailer(len(items), lr.Total, n)
			return nil
		},
	}
	cmd.Flags().Bool("self", false, "list GPG keys for the authenticated user")
	return cmd
}

func newUserGPGAddCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add --self --armored=<key-or-@file>",
		Short: "Add a GPG key",
		Long:  "Add a GPG key to the authenticated account. This is self-only; passing a user is an error because Forgejo has no admin-on-behalf endpoint. --armored is a literal armored public key unless it starts with @, in which case the rest is read as a file path and trailing newlines are stripped.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target, self, err := keyTarget(cmd, args, "Usage: forgejo user gpg add --self --armored=<key-or-@file>")
			if err != nil {
				return err
			}
			if !self || target != "" {
				return cmdutil.Usagef("GPG add is self-only (no admin-on-behalf endpoint in Forgejo).\n        Run as the target user with --self.")
			}
			armored, _ := cmd.Flags().GetString("armored")
			if armored == "" {
				return cmdutil.Usagef("Missing --armored (literal or @file)")
			}
			content, err := readKeyArg(armored)
			if err != nil {
				return err
			}
			body, err := json.Marshal(map[string]any{"armored_public_key": content})
			if err != nil {
				return err
			}
			raw, err := ctx.Client.Do("POST", "user/gpg_keys", body)
			if err != nil {
				return err
			}
			if ctx.WantsJSON() {
				return ctx.EmitJSON(raw)
			}
			obj, err := cmdutil.ParseObject(raw)
			if err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Added GPG key: %s (id=%s)\n", cmdutil.Str(obj, "key_id"), cmdutil.Str(obj, "id"))
			return nil
		},
	}
	cmd.Flags().Bool("self", false, "add the GPG key to the authenticated user (required)")
	cmd.Flags().String("armored", "", "literal armored GPG public key, or @path to read a key file (required)")
	return cmd
}

func newUserGPGDeleteCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete --self <id> [--yes]",
		Short: "Delete a GPG key",
		Long:  "Delete a GPG key from the authenticated account by numeric id. This is self-only; passing a user is an error because Forgejo has no admin-on-behalf endpoint. This destructive command requires --yes or an interactive typed-name confirmation.",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			target, self, keyID, err := keyTargetID(cmd, args, "Usage: forgejo user gpg delete <user|--self> <id> [--yes]")
			if err != nil {
				return err
			}
			if !self || target != "" {
				return cmdutil.Usagef("GPG delete is self-only (no admin-on-behalf endpoint in Forgejo).")
			}
			keyID, err = cmdutil.IDArg(keyID, "GPG key id")
			if err != nil {
				return err
			}
			if err := ctx.ConfirmDelete(cmd, "GPG key", keyID); err != nil {
				return err
			}
			if _, err := ctx.Client.Do("DELETE", "user/gpg_keys/"+keyID, nil); err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Deleted GPG key: %s\n", keyID)
			return nil
		},
	}
	cmd.Flags().Bool("self", false, "delete the GPG key from the authenticated user (required)")
	cmdutil.AddYesFlag(cmd)
	return cmd
}

func newUserTokenCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token <list|create|delete>",
		Short: "Manage access tokens",
		Long:  "Manage a user's access tokens. Forgejo requires HTTP Basic auth for users/{user}/tokens, so these verbs use the username argument plus FORGEJO_PASSWORD from the config instead of bearer-token auth. --otp sends X-Forgejo-OTP for accounts with TOTP enabled.",
	}
	cmd.AddCommand(newUserTokenListCmd(ctx), newUserTokenCreateCmd(ctx), newUserTokenDeleteCmd(ctx))
	return cmd
}

func newUserTokenListCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <user> [--otp=X]",
		Short: "List access tokens",
		Long:  "List access tokens for a user using HTTP Basic auth. FORGEJO_PASSWORD must be set in the config. --otp is passed as X-Forgejo-OTP for accounts with TOTP enabled. The bash command fetched ?limit=50; the global --limit flag overrides that page size, and --limit=0 fetches all pages.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			user := args[0]
			otp, _ := cmd.Flags().GetString("otp")
			n := ctx.ListLimit(50)
			lr, err := doBasicList(ctx, "users/"+cmdutil.NameSeg(user)+"/tokens", user, otp, n)
			if err != nil {
				return err
			}
			if ctx.WantsJSON() {
				return ctx.EmitJSON(lr.Body)
			}
			items, err := cmdutil.ParseArray(lr.Body)
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(items))
			for _, m := range items {
				rows = append(rows, []string{
					cmdutil.Str(m, "id"),
					cmdutil.Str(m, "name"),
					cmdutil.Str(m, "scopes"),
				})
			}
			ctx.Table([]string{"ID", "NAME", "SCOPES"}, rows)
			ctx.Trailer(len(items), lr.Total, n)
			return nil
		},
	}
	cmd.Flags().String("otp", "", "one-time password for TOTP accounts, sent as X-Forgejo-OTP")
	return cmd
}

func newUserTokenCreateCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <user> --name=X [--scopes=read:repo,...] [--otp=X]",
		Short: "Create an access token",
		Long: "Create an access token for a user using HTTP Basic auth. FORGEJO_PASSWORD must be set in the config. --otp is passed as X-Forgejo-OTP for accounts with TOTP enabled.\n\n" +
			"--name is required. --scopes is optional; when omitted the request body contains only the token name. In text mode the token sha1 is printed once with the bash warning because it cannot be retrieved again.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			user := args[0]
			name, _ := cmd.Flags().GetString("name")
			scopes, _ := cmd.Flags().GetString("scopes")
			otp, _ := cmd.Flags().GetString("otp")
			if name == "" {
				return cmdutil.Usagef("Missing --name")
			}
			bodyMap := map[string]any{"name": name}
			if scopes != "" {
				bodyMap["scopes"] = strings.Split(scopes, ",")
			}
			body, err := json.Marshal(bodyMap)
			if err != nil {
				return err
			}
			raw, err := doBasic(ctx, "POST", "users/"+cmdutil.NameSeg(user)+"/tokens", user, otp, body)
			if err != nil {
				return err
			}
			if ctx.WantsJSON() {
				return ctx.EmitJSON(raw)
			}
			obj, err := cmdutil.ParseObject(raw)
			if err != nil {
				return err
			}
			fmt.Fprintln(ctx.Err, "WARNING: token shown ONCE below \u2014 copy it now.")
			fmt.Fprintf(ctx.Out, "Token: %s\nID:    %s\nName:  %s\n", cmdutil.Str(obj, "sha1"), cmdutil.Str(obj, "id"), cmdutil.Str(obj, "name"))
			return nil
		},
	}
	cmd.Flags().String("name", "", "access token name (required)")
	cmd.Flags().String("scopes", "", "comma-separated token scopes; omitted lets the server choose its default")
	cmd.Flags().String("otp", "", "one-time password for TOTP accounts, sent as X-Forgejo-OTP")
	return cmd
}

func newUserTokenDeleteCmd(ctx *cmdutil.Ctx) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <user> <name> [--otp=X] [--yes]",
		Short: "Delete an access token",
		Long:  "Delete an access token by name using HTTP Basic auth. FORGEJO_PASSWORD must be set in the config. --otp is passed as X-Forgejo-OTP for accounts with TOTP enabled. This destructive command requires --yes or an interactive typed-name confirmation.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			user, name := args[0], args[1]
			otp, _ := cmd.Flags().GetString("otp")
			if err := ctx.ConfirmDelete(cmd, "access token", name); err != nil {
				return err
			}
			if _, err := doBasic(ctx, "DELETE", "users/"+cmdutil.NameSeg(user)+"/tokens/"+cmdutil.NameSeg(name), user, otp, nil); err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "Deleted access token: %s\n", name)
			return nil
		},
	}
	cmd.Flags().String("otp", "", "one-time password for TOTP accounts, sent as X-Forgejo-OTP")
	cmdutil.AddYesFlag(cmd)
	return cmd
}

func keyTarget(cmd *cobra.Command, args []string, usage string) (string, bool, error) {
	self, _ := cmd.Flags().GetBool("self")
	if self {
		if len(args) != 0 {
			return "", false, cmdutil.Usagef("pass either <user> or --self, not both")
		}
		return "", true, nil
	}
	if len(args) != 1 {
		return "", false, cmdutil.Usagef("%s", usage)
	}
	return args[0], false, nil
}

func keyTargetID(cmd *cobra.Command, args []string, usage string) (string, bool, string, error) {
	self, _ := cmd.Flags().GetBool("self")
	if self {
		if len(args) != 1 {
			return "", false, "", cmdutil.Usagef("%s", usage)
		}
		return "", true, args[0], nil
	}
	if len(args) != 2 {
		return "", false, "", cmdutil.Usagef("%s", usage)
	}
	return args[0], false, args[1], nil
}

func readKeyArg(val string) (string, error) {
	if !strings.HasPrefix(val, "@") {
		return val, nil
	}
	path := val[1:]
	st, err := os.Stat(path)
	if err != nil || !st.Mode().IsRegular() {
		return "", cmdutil.Usagef("Key file not found: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(data), "\n"), nil
}

type basicListResult struct {
	Body  []byte
	Total int
}

func doBasicList(ctx *cmdutil.Ctx, endpoint, username, otp string, limit int) (*basicListResult, error) {
	if limit <= 0 {
		body, err := doBasicPaged(ctx, endpoint, username, otp)
		if err != nil {
			return nil, err
		}
		return &basicListResult{Body: body, Total: -1}, nil
	}
	raw, err := doBasic(ctx, "GET", endpointWithLimit(endpoint, limit, 1), username, otp, nil)
	if err != nil {
		return nil, err
	}
	return &basicListResult{Body: raw, Total: -1}, nil
}

func doBasicPaged(ctx *cmdutil.Ctx, endpoint, username, otp string) ([]byte, error) {
	seen := map[string]bool{}
	var acc []json.RawMessage
	for page := 1; ; page++ {
		raw, err := doBasic(ctx, "GET", endpointWithLimit(endpoint, 50, page), username, otp, nil)
		if err != nil {
			return nil, err
		}
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, fmt.Errorf("expected JSON array from %s: %w", endpoint, err)
		}
		added := 0
		for _, it := range items {
			key := string(it)
			var probe struct {
				ID json.Number `json:"id"`
			}
			if err := json.Unmarshal(it, &probe); err == nil && probe.ID != "" {
				key = probe.ID.String()
			}
			if !seen[key] {
				seen[key] = true
				acc = append(acc, it)
				added++
			}
		}
		if added == 0 || len(items) < 50 {
			break
		}
		if page >= 200 {
			fmt.Fprintf(ctx.Err, "forgejo: warning: pagination cap (200 pages) hit at %s\n", endpoint)
			break
		}
	}
	if acc == nil {
		return []byte("[]"), nil
	}
	sort.SliceStable(acc, func(i, j int) bool {
		return rawIDSortKey(acc[i]) < rawIDSortKey(acc[j])
	})
	return json.Marshal(acc)
}

func endpointWithLimit(endpoint string, limit, page int) string {
	sep := "?"
	if strings.Contains(endpoint, "?") {
		sep = "&"
	}
	return fmt.Sprintf("%s%slimit=%d&page=%d", endpoint, sep, limit, page)
}

func rawIDSortKey(item json.RawMessage) float64 {
	var probe struct {
		ID json.Number `json:"id"`
	}
	if err := json.Unmarshal(item, &probe); err == nil && probe.ID != "" {
		if f, err := probe.ID.Float64(); err == nil {
			return f
		}
	}
	return 0
}

func doBasic(ctx *cmdutil.Ctx, method, endpoint, username, otp string, body []byte) ([]byte, error) {
	password := ""
	if ctx.Config != nil {
		password = ctx.Config.Password
	}
	if password == "" {
		return nil, fmt.Errorf("FORGEJO_PASSWORD is not set in %s.\nForgejo's token endpoints require basic auth, not bearer.\nAdd to your config: FORGEJO_PASSWORD=your-account-password", config.Path())
	}
	status, raw, err := ctx.Client.DoBasic(method, endpoint, username, password, otp, body)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, basicAPIError(status, raw)
	}
	return raw, nil
}

func basicAPIError(status int, body []byte) *api.Error {
	msg := strings.TrimSpace(string(body))
	var parsed struct {
		Message string `json:"message"`
		Err     string `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil {
		if parsed.Message != "" {
			msg = parsed.Message
		} else if parsed.Err != "" {
			msg = parsed.Err
		}
	}
	if msg == "" {
		msg = http.StatusText(status)
	}
	return &api.Error{Status: status, Message: msg}
}

func date10(s string) string {
	if len(s) > 10 {
		return s[:10]
	}
	return s
}

func strDefault(m map[string]any, path, def string) string {
	v, ok := lookupPath(m, path)
	if !ok || v == nil {
		return def
	}
	return scalarString(v)
}

func numberDefault(m map[string]any, path string, def int) any {
	v, ok := lookupPath(m, path)
	if !ok || v == nil {
		return def
	}
	switch t := v.(type) {
	case json.Number:
		return t
	case int:
		return t
	case float64:
		return t
	case string:
		if t == "" {
			return def
		}
		if i, err := strconv.Atoi(t); err == nil {
			return i
		}
	}
	return def
}

func lookupPath(m map[string]any, path string) (any, bool) {
	var cur any = m
	for _, part := range strings.Split(path, ".") {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = obj[part]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

func bashBool(s string) bool {
	return s == "true"
}

func gpgEmails(m map[string]any) string {
	v, ok := lookupPath(m, "emails")
	if !ok {
		return ""
	}
	items, _ := v.([]any)
	out := make([]string, 0, len(items))
	for _, item := range items {
		obj, _ := item.(map[string]any)
		if email := scalarString(obj["email"]); email != "" {
			out = append(out, email)
		}
	}
	return strings.Join(out, ",")
}
