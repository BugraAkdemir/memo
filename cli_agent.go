package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"memo/internal/replcli"
)

// runAgentCommand implements `memo agent <verb> [flags]` — CLI management of
// agent (tool-use) mode and its Shift+Tab auto-permission toggle, same shape
// as `memo remote`/`memo provider`. Exists because toggling these on a
// headless self-hosted install otherwise required a raw curl against
// /api/agent/enabled and /api/agent/auto-permission.
func runAgentCommand(args []string) int {
	if len(args) < 1 {
		printAgentUsage()
		return 1
	}
	verb := args[0]

	fs := flag.NewFlagSet("agent "+verb, flag.ContinueOnError)
	fs.Usage = printAgentUsage
	port := fs.Int("port", 8090, "Backend port")
	token := fs.String("token", "", "Device or session token — required if the backend was started with --lan")
	flagArgs, positional := splitFlagsAndPositional(args[1:], nil)
	if err := fs.Parse(flagArgs); err != nil {
		return 1
	}

	client := replcli.NewClient(fmt.Sprintf("http://127.0.0.1:%d", *port))
	if *token != "" {
		client.SetToken(*token)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	switch verb {
	case "status":
		return agentStatusCmd(ctx, client)
	case "enable":
		return agentSetEnabledCmd(ctx, client, true)
	case "disable":
		return agentSetEnabledCmd(ctx, client, false)
	case "auto-permission":
		if len(positional) != 1 {
			fmt.Fprintln(os.Stderr, "kullanım / usage: memo agent auto-permission <status|on|off>")
			return 1
		}
		switch positional[0] {
		case "status":
			return agentAutoPermissionStatusCmd(ctx, client)
		case "on":
			return agentSetAutoPermissionCmd(ctx, client, true)
		case "off":
			return agentSetAutoPermissionCmd(ctx, client, false)
		default:
			fmt.Fprintln(os.Stderr, "kullanım / usage: memo agent auto-permission <status|on|off>")
			return 1
		}
	default:
		printAgentUsage()
		return 1
	}
}

func printAgentUsage() {
	fmt.Fprintln(os.Stderr, `kullanım / usage:
  memo agent status
  memo agent enable
  memo agent disable
  memo agent auto-permission status|on|off

her komut / every command: [--port N] [--token T]
  --lan ile başlatılmış bir backend her istekte kimlik ister — o zaman
  --token (mevcut bir cihaz/oturum token'ı) gerekir.
  a backend started with --lan requires a credential on every request —
  pass --token (an existing device/session token) in that case.

  auto-permission açıkken hiçbir tool çağrısı onay istemez — DİKKATLİ
  kullanın, agent dosya sistemi/shell üzerinde insan onayı olmadan işlem
  yapabilir.
  with auto-permission on, no tool call ever asks for approval — use
  CAREFULLY, the agent can act on the filesystem/shell with zero human
  review.`)
}

func agentStatusCmd(ctx context.Context, c *replcli.Client) int {
	enabled, err := c.GetAgentEnabled(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "durum alınamadı / failed to get status: %v\n", err)
		hintIfUnauthorized(err)
		return 1
	}
	auto, err := c.GetAgentAutoPermission(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "auto-permission durumu alınamadı / failed to get auto-permission status: %v\n", err)
		return 1
	}
	fmt.Printf("Agent modu / agent mode: %v\n", enabled)
	fmt.Printf("Auto-permission: %v\n", auto)
	return 0
}

func agentSetEnabledCmd(ctx context.Context, c *replcli.Client, enabled bool) int {
	if err := c.SetAgentEnabled(ctx, enabled); err != nil {
		fmt.Fprintf(os.Stderr, "agent modu ayarlanamadı / failed to set agent mode: %v\n", err)
		hintIfUnauthorized(err)
		return 1
	}
	fmt.Printf("✓ Agent modu / agent mode: %v\n", enabled)
	return 0
}

func agentAutoPermissionStatusCmd(ctx context.Context, c *replcli.Client) int {
	auto, err := c.GetAgentAutoPermission(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "durum alınamadı / failed to get status: %v\n", err)
		hintIfUnauthorized(err)
		return 1
	}
	fmt.Printf("Auto-permission: %v\n", auto)
	return 0
}

func agentSetAutoPermissionCmd(ctx context.Context, c *replcli.Client, enabled bool) int {
	if err := c.SetAgentAutoPermission(ctx, enabled); err != nil {
		fmt.Fprintf(os.Stderr, "auto-permission ayarlanamadı / failed to set auto-permission: %v\n", err)
		hintIfUnauthorized(err)
		return 1
	}
	fmt.Printf("✓ Auto-permission: %v\n", enabled)
	return 0
}
