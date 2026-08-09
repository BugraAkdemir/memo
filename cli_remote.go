package main

import (
	"context"
	"flag"
	"fmt"
	"os"
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
	password := fs.String("password", "", "Password (set-mode/login)")
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
		return remoteSetModeCmd(ctx, client, positional[0], *username, *password)
	case "login":
		if *username == "" || *password == "" {
			fmt.Fprintln(os.Stderr, "kullanım / usage: memo remote login --username X --password Y")
			return 1
		}
		return remoteLoginCmd(ctx, client, *username, *password)
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
  memo remote set-mode <none|token|password|token_password> [--username X] [--password Y]
  memo remote login --username X --password Y

her komut / every command: [--port N] [--token T]
  --lan ile başlatılmış bir backend her istekte kimlik ister — o zaman
  --token (mevcut bir cihaz/oturum token'ı) gerekir.
  a backend started with --lan requires a credential on every request —
  pass --token (an existing device/session token) in that case.`)
}

func remoteStatusCmd(ctx context.Context, c *replcli.Client) int {
	status, err := c.RemoteFullAccessStatus(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "durum alınamadı / failed to get status: %v\n", err)
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
		return 1
	}
	fmt.Printf("✓ Cihaz eklendi / device added: %s\n", name)
	fmt.Printf("Token (yalnızca şimdi gösteriliyor / shown only now): %s\n", token)
	return 0
}

func remoteRevokeDeviceCmd(ctx context.Context, c *replcli.Client, id string) int {
	if err := c.RevokeRemoteDevice(ctx, id); err != nil {
		fmt.Fprintf(os.Stderr, "cihaz kaldırılamadı / failed to revoke device: %v\n", err)
		return 1
	}
	fmt.Println("✓ Cihaz erişimi kaldırıldı / device access revoked.")
	return 0
}

func remoteSetModeCmd(ctx context.Context, c *replcli.Client, mode, username, password string) int {
	if err := c.SetRemoteAuthMode(ctx, mode, username, password); err != nil {
		fmt.Fprintf(os.Stderr, "auth modu ayarlanamadı / failed to set auth mode: %v\n", err)
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
