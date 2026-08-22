package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"memo/internal/replcli"
)

// runRemoteCommand implements `memo remote <verb> [flags]` — CLI management
// of remote-access auth (Faz 2/3, yapacam.md), talking to a running
// backend's REST API rather than editing config.yaml directly (unlike
// `memo config`): auth mode changes go through app.SetRemoteAuthConfig's
// validation, device tokens are only ever visible once at creation, exactly
// like the Settings UI — this is a second client of the same endpoints, not
// a separate code path.
func runRemoteCommand(args []string) int {
	if len(args) < 1 {
		printRemoteUsage()
		return 1
	}
	verb := args[0]

	fs := flag.NewFlagSet("remote "+verb, flag.ContinueOnError)
	fs.Usage = printRemoteUsage
	port := fs.Int("port", 8090, "Backend port")
	token := fs.String("token", "", "Device or session token — required if the backend was started with --lan (0.0.0.0 bind requires a credential on every request, including from this same machine)")
	username := fs.String("username", "", "Username (set-mode/login)")
	password := fs.String("password", "", "Password (set-mode/login, add-account)")
	role := fs.String("role", "user", "Account role for add-account: admin|user")
	perm := fs.String("perm", "", "Comma-separated permission checkboxes for add-account (Faz 5.1.1, only meaningful for --role user): models,memory,agent,calendar,whatsapp,telegram,routines")
	// No boolean flags in this subcommand — every flag here takes a value.
	flagArgs, positional := splitFlagsAndPositional(args[1:], nil)
	if err := fs.Parse(flagArgs); err != nil {
		return 1 // flag package already printed the error
	}

	client := replcli.NewClient(fmt.Sprintf("http://127.0.0.1:%d", *port))
	if *token != "" {
		client.SetToken(*token)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	switch verb {
	case "status":
		return remoteStatusCmd(ctx, client)
	case "list-devices":
		return remoteListDevicesCmd(ctx, client)
	case "add-device":
		if len(positional) != 1 {
			fmt.Fprintln(os.Stderr, "kullanım / usage: memo remote add-device <ad/name>")
			return 1
		}
		return remoteAddDeviceCmd(ctx, client, positional[0])
	case "revoke-device":
		if len(positional) != 1 {
			fmt.Fprintln(os.Stderr, "kullanım / usage: memo remote revoke-device <id>")
			return 1
		}
		return remoteRevokeDeviceCmd(ctx, client, positional[0])
	case "set-mode":
		if len(positional) != 1 {
			fmt.Fprintln(os.Stderr, "kullanım / usage: memo remote set-mode <none|token|password|token_password> [--username X] [--password Y]")
			return 1
		}
		pw := *password
		if pw == "" && (positional[0] == "password" || positional[0] == "token_password") {
			// Only prompt for password-backed modes — "none"/"token" have no
			// use for one, and an empty password here means "leave any
			// existing hash untouched" server-side (see
			// app.SetRemoteAuthConfig), a real, valid choice for those modes.
			var err error
			pw, err = promptSecret("Şifre / Password")
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
		}
		return remoteSetModeCmd(ctx, client, positional[0], *username, pw)
	case "login":
		if *username == "" {
			fmt.Fprintln(os.Stderr, "kullanım / usage: memo remote login --username X [--password Y]")
			return 1
		}
		pw := *password
		if pw == "" {
			var err error
			pw, err = promptSecret("Şifre / Password")
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
		}
		return remoteLoginCmd(ctx, client, *username, pw)
	case "rotate-token":
		if len(positional) != 1 {
			fmt.Fprintln(os.Stderr, "kullanım / usage: memo remote rotate-token <cihaz id/device id>")
			return 1
		}
		return remoteRotateTokenCmd(ctx, client, positional[0])
	case "list-accounts":
		return remoteListAccountsCmd(ctx, client)
	case "add-account":
		if len(positional) != 1 {
			fmt.Fprintln(os.Stderr, "kullanım / usage: memo remote add-account <kullanıcı adı/username> [--role admin|user] [--password P] [--perm models,memory,...]")
			return 1
		}
		if *role != "admin" && *role != "user" {
			fmt.Fprintln(os.Stderr, "kullanım / usage: --role admin|user")
			return 1
		}
		perms, err := parsePermissions(*perm)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		pw := *password
		if pw == "" {
			var err error
			pw, err = promptSecret("Şifre / Password")
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
		}
		return remoteAddAccountCmd(ctx, client, positional[0], pw, *role, perms)
	case "delete-account":
		if len(positional) != 1 {
			fmt.Fprintln(os.Stderr, "kullanım / usage: memo remote delete-account <id>")
			return 1
		}
		return remoteDeleteAccountCmd(ctx, client, positional[0])
	default:
		printRemoteUsage()
		return 1
	}
}

func printRemoteUsage() {
	fmt.Fprintln(os.Stderr, `kullanım / usage:
  memo remote status
  memo remote list-devices
  memo remote add-device <ad/name>
  memo remote revoke-device <id>
  memo remote rotate-token <id>
  memo remote set-mode <none|token|password|token_password> [--username X] [--password Y]
  memo remote login --username X [--password Y]
  memo remote list-accounts
  memo remote add-account <kullanıcı adı/username> [--role admin|user] [--password P] [--perm a,b,c]
  memo remote delete-account <id>

her komut / every command: [--port N] [--token T]
  --lan ile başlatılmış bir backend her istekte kimlik ister — o zaman
  --token (mevcut bir cihaz/oturum token'ı) gerekir.
  a backend started with --lan requires a credential on every request —
  pass --token (an existing device/session token) in that case.

  --password verilmezse, set-mode (password/token_password), login ve
  add-account terminalden gizli (görünmez) olarak sorar.
  if --password is omitted, set-mode (password/token_password), login and
  add-account prompt for it interactively, hidden, instead.

  add-account --role varsayılan / defaults to "user" (Faz 5.1.1 — en
  kısıtlı, hiçbir kutu işaretlenmemiş hesap). --perm, sadece --role user
  için anlamlıdır (admin zaten her şeye sahiptir), virgülle ayrılmış:
  models,memory,agent,calendar,whatsapp,telegram,routines
  add-account --role defaults to "user" (Faz 5.1.1 — most restrictive,
  nothing checked). --perm only matters for --role user (admin already has
  everything), comma-separated from the same seven names above.`)
}

// hintIfUnauthorized appends a pointer to the systemd journal when the
// error looks like a 401 — the single most common first-run confusion:
// once bound to 0.0.0.0 (--lan), the API requires a credential for EVERY
// request, including these CLI calls run locally on the same box, so
// there's no bootstrap "just ask the CLI" path for a freshly auto-
// generated device token. It's only ever visible in the backend's own
// process log (main.go's --lan startup line), which under systemd means
// the journal, not this terminal.
func hintIfUnauthorized(err error) {
	if err == nil || !strings.Contains(err.Error(), "401") {
		return
	}
	fmt.Fprintln(os.Stderr, "İpucu / Hint: --lan ile ilk kurulumda üretilen token'ı görmek için:")
	fmt.Fprintln(os.Stderr, "  journalctl --user -u memo.service --no-pager | grep -i token")
	fmt.Fprintln(os.Stderr, "Ya da --token ile geçerli bir cihaz/oturum token'ı ver, veya `memo remote login` kullan.")
}

func remoteStatusCmd(ctx context.Context, c *replcli.Client) int {
	status, err := c.RemoteFullAccessStatus(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "durum alınamadı / failed to get status: %v\n", err)
		hintIfUnauthorized(err)
		return 1
	}
	fmt.Printf("Etkin / Enabled: %v\n", status.Enabled)
	fmt.Printf("Çalışıyor / Running: %v\n", status.Running)
	fmt.Printf("Auth modu / Auth mode: %s\n", status.AuthMode)
	if status.Username != "" {
		fmt.Printf("Kullanıcı adı / Username: %s\n", status.Username)
	}
	for _, addr := range status.Addresses {
		fmt.Printf("Adres / Address: %s\n", addr)
	}
	if status.AuthWarning != "" {
		fmt.Printf("⚠️  %s\n", status.AuthWarning)
	}
	return 0
}

func remoteListDevicesCmd(ctx context.Context, c *replcli.Client) int {
	devices, err := c.ListRemoteDevices(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cihazlar alınamadı / failed to list devices: %v\n", err)
		hintIfUnauthorized(err)
		return 1
	}
	if len(devices) == 0 {
		fmt.Println("Henüz eşleşmiş cihaz yok. / No paired devices yet.")
		return 0
	}
	for _, d := range devices {
		lastSeen := "hiç kullanılmadı / never used"
		if d.LastSeenAt != nil {
			lastSeen = d.LastSeenAt.Format(time.RFC3339)
		}
		fmt.Printf("%s\t%s\t(son görülme / last seen: %s)\n", d.ID, d.Name, lastSeen)
	}
	return 0
}

func remoteAddDeviceCmd(ctx context.Context, c *replcli.Client, name string) int {
	token, err := c.AddRemoteDevice(ctx, name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cihaz eklenemedi / failed to add device: %v\n", err)
		hintIfUnauthorized(err)
		return 1
	}
	fmt.Printf("✓ Cihaz eklendi / device added: %s\n", name)
	fmt.Printf("Token (yalnızca şimdi gösteriliyor / shown only now): %s\n", token)
	return 0
}

func remoteRevokeDeviceCmd(ctx context.Context, c *replcli.Client, id string) int {
	if err := c.RevokeRemoteDevice(ctx, id); err != nil {
		fmt.Fprintf(os.Stderr, "cihaz kaldırılamadı / failed to revoke device: %v\n", err)
		hintIfUnauthorized(err)
		return 1
	}
	fmt.Println("✓ Cihaz erişimi kaldırıldı / device access revoked.")
	return 0
}

// remoteRotateTokenCmd revokes device id and immediately re-adds it under
// the same name, printing the freshly minted token. There's no dedicated
// "reissue" endpoint — device tokens are deliberately write-once (see
// AddRemoteDevice's doc comment), so rotation is revoke-then-recreate, same
// as a user doing it by hand through list-devices/revoke-device/add-device,
// just as one command. Useful when a token may have leaked (shared over an
// insecure channel, visible in a log) and needs to stop working immediately
// without losing the device's identity/name.
func remoteRotateTokenCmd(ctx context.Context, c *replcli.Client, id string) int {
	devices, err := c.ListRemoteDevices(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cihazlar alınamadı / failed to list devices: %v\n", err)
		hintIfUnauthorized(err)
		return 1
	}
	var name string
	found := false
	for _, d := range devices {
		if d.ID == id {
			name = d.Name
			found = true
			break
		}
	}
	if !found {
		fmt.Fprintf(os.Stderr, "cihaz bulunamadı / device not found: %s\n", id)
		return 1
	}

	if err := c.RevokeRemoteDevice(ctx, id); err != nil {
		fmt.Fprintf(os.Stderr, "eski token iptal edilemedi / failed to revoke the old token: %v\n", err)
		hintIfUnauthorized(err)
		return 1
	}
	token, err := c.AddRemoteDevice(ctx, name)
	if err != nil {
		// Old token is already revoked at this point — say so clearly rather
		// than leaving the device with no working token at all and no
		// explanation.
		fmt.Fprintf(os.Stderr, "eski token iptal edildi ama yenisi oluşturulamadı / old token revoked but a new one could not be created: %v\n", err)
		fmt.Fprintln(os.Stderr, "cihazı elle yeniden ekleyin / re-add the device by hand: memo remote add-device \""+name+"\"")
		return 1
	}
	fmt.Printf("✓ Token yenilendi / token rotated: %s\n", name)
	fmt.Printf("Yeni token (yalnızca şimdi gösteriliyor / new token, shown only now): %s\n", token)
	return 0
}

func remoteSetModeCmd(ctx context.Context, c *replcli.Client, mode, username, password string) int {
	if err := c.SetRemoteAuthMode(ctx, mode, username, password); err != nil {
		fmt.Fprintf(os.Stderr, "auth modu ayarlanamadı / failed to set auth mode: %v\n", err)
		hintIfUnauthorized(err)
		return 1
	}
	fmt.Printf("✓ Auth modu ayarlandı / auth mode set: %s\n", mode)
	return 0
}

func remoteLoginCmd(ctx context.Context, c *replcli.Client, username, password string) int {
	token, err := c.LoginRemote(ctx, username, password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "giriş başarısız / login failed: %v\n", err)
		return 1
	}
	fmt.Printf("✓ Giriş başarılı / login successful. Session token: %s\n", token)
	fmt.Println("Bu token'ı sonraki komutlarda --token ile kullan. / Use this token with --token in later commands.")
	return 0
}

// permissionNames lists every recognized --perm token, in the same order
// AccountPermissions' fields are declared — shared between parsePermissions
// (validation) and permissionsSummary (display).
var permissionNames = []string{"models", "memory", "agent", "calendar", "whatsapp", "telegram", "routines"}

// parsePermissions turns a comma-separated --perm value (e.g.
// "models,memory") into a replcli.AccountPermissions, rejecting anything
// that isn't one of permissionNames outright rather than silently ignoring
// a typo — a permission a human thinks they granted but didn't (e.g.
// "--perm modle" instead of "models") is a much worse failure mode than a
// command that refuses to run at all.
func parsePermissions(csv string) (replcli.AccountPermissions, error) {
	var perms replcli.AccountPermissions
	if csv == "" {
		return perms, nil
	}
	for _, name := range strings.Split(csv, ",") {
		name = strings.TrimSpace(name)
		switch name {
		case "models":
			perms.Models = true
		case "memory":
			perms.Memory = true
		case "agent":
			perms.Agent = true
		case "calendar":
			perms.Calendar = true
		case "whatsapp":
			perms.WhatsApp = true
		case "telegram":
			perms.Telegram = true
		case "routines":
			perms.Routines = true
		default:
			return perms, fmt.Errorf("bilinmeyen izin / unknown permission: %q (geçerli / valid: %s)", name, strings.Join(permissionNames, ", "))
		}
	}
	return perms, nil
}

// permissionsSummary formats an account's granted permissions for display
// (list-accounts) — "hepsi/all" for admin (Permissions is never consulted
// for that role, see config.Account.Permissions' doc comment, so showing
// its raw checkbox state would be misleading), "yok/none" for a "user"
// account with nothing checked, otherwise the checked names joined.
func permissionsSummary(role string, p replcli.AccountPermissions) string {
	if role != "user" {
		return "hepsi / all"
	}
	var granted []string
	if p.Models {
		granted = append(granted, "models")
	}
	if p.Memory {
		granted = append(granted, "memory")
	}
	if p.Agent {
		granted = append(granted, "agent")
	}
	if p.Calendar {
		granted = append(granted, "calendar")
	}
	if p.WhatsApp {
		granted = append(granted, "whatsapp")
	}
	if p.Telegram {
		granted = append(granted, "telegram")
	}
	if p.Routines {
		granted = append(granted, "routines")
	}
	if len(granted) == 0 {
		return "yok (sadece sohbet) / none (chat only)"
	}
	return strings.Join(granted, ",")
}

func remoteListAccountsCmd(ctx context.Context, c *replcli.Client) int {
	accounts, err := c.ListAccounts(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hesaplar alınamadı / failed to list accounts: %v\n", err)
		hintIfUnauthorized(err)
		return 1
	}
	if len(accounts) == 0 {
		fmt.Println("Henüz hesap yok. / No accounts yet.")
		return 0
	}
	for _, a := range accounts {
		fmt.Printf("%s\t%s\t%s\tizinler/permissions: %s\n", a.ID, a.Username, a.Role, permissionsSummary(a.Role, a.Permissions))
	}
	return 0
}

func remoteAddAccountCmd(ctx context.Context, c *replcli.Client, username, password, role string, perms replcli.AccountPermissions) int {
	if err := c.CreateAccount(ctx, username, password, role, perms); err != nil {
		fmt.Fprintf(os.Stderr, "hesap eklenemedi / failed to add account: %v\n", err)
		hintIfUnauthorized(err)
		return 1
	}
	fmt.Printf("✓ Hesap eklendi / account added: %s (%s)\n", username, role)
	if role == "user" {
		fmt.Printf("İzinler / permissions: %s\n", permissionsSummary(role, perms))
	}
	return 0
}

func remoteDeleteAccountCmd(ctx context.Context, c *replcli.Client, id string) int {
	if err := c.DeleteAccount(ctx, id); err != nil {
		fmt.Fprintf(os.Stderr, "hesap silinemedi / failed to delete account: %v\n", err)
		hintIfUnauthorized(err)
		return 1
	}
	fmt.Println("✓ Hesap silindi / account deleted.")
	return 0
}
