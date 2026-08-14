package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"flag"

	"memo/internal/replcli"
)

// runProviderCommand implements `memo provider <verb> [flags]` — CLI
// management of external LLM providers, same shape as `memo remote`
// (cli_remote.go): a second client of the existing /api/providers* REST
// endpoints, not a separate code path. Exists because setting up an
// external provider on a headless self-hosted install (SSH-only, no
// Flutter GUI) otherwise had no CLI path at all — the only way was a raw
// curl against /api/providers.
func runProviderCommand(args []string) int {
	if len(args) < 1 {
		printProviderUsage()
		return 1
	}
	verb := args[0]

	fs := flag.NewFlagSet("provider "+verb, flag.ContinueOnError)
	fs.Usage = printProviderUsage
	port := fs.Int("port", 8090, "Backend port")
	token := fs.String("token", "", "Device or session token — required if the backend was started with --lan")
	ptype := fs.String("type", "", "Sağlayıcı tipi / provider type (openai, claude, gemini, grok, groq, openrouter, ollama, opencode-zen, opencode-go, custom)")
	name := fs.String("name", "", "Görünen ad — set-active/list için benzersiz kimlik olarak kullanılır / display name — used as the unique identifier in set-active/list")
	model := fs.String("model", "", "Kullanılacak model / model name to use")
	baseURL := fs.String("base-url", "", "Sağlayıcının varsayılan API adresini geçersiz kıl / override the provider's default API base URL")
	key := fs.String("key", "", "API key. Boş bırakılırsa gizli olarak sorulur — gerçek bir key'i asla shell history/ps çıktısında bırakmayın / omit to be prompted for it hidden — never leave a real key in shell history/ps output")
	priority := fs.Int("priority", 0, "Fallback sırası — düşük olan önce denenir / fallback priority — lower runs first")
	enable := fs.Bool("enable", true, "Bu sağlayıcıyı hemen etkinleştir / enable this provider immediately")
	activate := fs.Bool("activate", false, "Bunu aktif sağlayıcı da yap / also make this the active provider")

	flagArgs, positional := splitFlagsAndPositional(args[1:], map[string]bool{"enable": true, "activate": true})
	if err := fs.Parse(flagArgs); err != nil {
		return 1 // flag package already printed the error
	}

	client := replcli.NewClient(fmt.Sprintf("http://127.0.0.1:%d", *port))
	if *token != "" {
		client.SetToken(*token)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	switch verb {
	case "list":
		return providerListCmd(ctx, client)
	case "add":
		if *ptype == "" || *name == "" {
			fmt.Fprintln(os.Stderr, "kullanım / usage: memo provider add --type T --name N [--model M] [--base-url URL] [--key K] [--priority N] [--enable=false] [--activate]")
			return 1
		}
		apiKey := *key
		if apiKey == "" {
			var err error
			apiKey, err = promptSecret("API key")
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
		}
		return providerAddCmd(ctx, client, *ptype, *name, *model, *baseURL, apiKey, *priority, *enable, *activate)
	case "set-active":
		if len(positional) != 1 {
			fmt.Fprintln(os.Stderr, "kullanım / usage: memo provider set-active <name>")
			return 1
		}
		return providerSetActiveCmd(ctx, client, positional[0])
	case "active":
		return providerActiveCmd(ctx, client)
	default:
		printProviderUsage()
		return 1
	}
}

func printProviderUsage() {
	fmt.Fprintln(os.Stderr, `kullanım / usage:
  memo provider list
  memo provider add --type T --name N [--model M] [--base-url URL] [--key K]
                     [--priority N] [--enable=false] [--activate]
  memo provider set-active <name>
  memo provider active

her komut / every command: [--port N] [--token T]
  --lan ile başlatılmış bir backend her istekte kimlik ister — o zaman
  --token (mevcut bir cihaz/oturum token'ı) gerekir.
  a backend started with --lan requires a credential on every request —
  pass --token (an existing device/session token) in that case.

  --key verilmezse terminalden gizli (görünmez) olarak sorulur.
  if --key is omitted, it is prompted for interactively, hidden.`)
}

func providerListCmd(ctx context.Context, c *replcli.Client) int {
	providers, err := c.ListProviders(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sağlayıcılar alınamadı / failed to list providers: %v\n", err)
		hintIfUnauthorized(err)
		return 1
	}
	activeName, _ := c.ActiveProviderName(ctx)
	if len(providers) == 0 {
		fmt.Println("Yapılandırılmış sağlayıcı yok. / No providers configured.")
		return 0
	}
	for _, p := range providers {
		mark := "  "
		if p.Name != "" && p.Name == activeName {
			mark = "→ "
		}
		state := "kapalı / disabled"
		if p.Enabled {
			state = "açık / enabled"
			if p.Connected {
				state += ", bağlı / connected"
			}
		}
		fmt.Printf("%s%s\t(%s)\tmodel=%s\tpriority=%d\t%s\n", mark, p.Name, p.Type, p.Model, p.Priority, state)
	}
	if activeName != "" {
		fmt.Printf("\nAktif / active: %s\n", activeName)
	}
	return 0
}

func providerAddCmd(ctx context.Context, c *replcli.Client, ptype, name, model, baseURL, apiKey string, priority int, enable, activate bool) int {
	cfg := replcli.ProviderConfig{
		Type:     ptype,
		Name:     name,
		Model:    model,
		BaseURL:  baseURL,
		APIKey:   apiKey,
		Priority: priority,
		Enabled:  enable,
	}
	if err := c.UpdateProvider(ctx, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "sağlayıcı eklenemedi / failed to add provider: %v\n", err)
		hintIfUnauthorized(err)
		return 1
	}
	fmt.Printf("✓ Sağlayıcı kaydedildi / provider saved: %s (%s)\n", name, ptype)
	if activate {
		if err := c.SetActiveProvider(ctx, name); err != nil {
			fmt.Fprintf(os.Stderr, "aktif sağlayıcı ayarlanamadı / failed to set active provider: %v\n", err)
			return 1
		}
		fmt.Printf("✓ Aktif sağlayıcı olarak ayarlandı / set as active provider: %s\n", name)
	}
	return 0
}

func providerSetActiveCmd(ctx context.Context, c *replcli.Client, name string) int {
	if err := c.SetActiveProvider(ctx, name); err != nil {
		fmt.Fprintf(os.Stderr, "aktif sağlayıcı ayarlanamadı / failed to set active provider: %v\n", err)
		hintIfUnauthorized(err)
		return 1
	}
	fmt.Printf("✓ Aktif sağlayıcı / active provider: %s\n", name)
	return 0
}

func providerActiveCmd(ctx context.Context, c *replcli.Client) int {
	name, err := c.ActiveProviderName(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "aktif sağlayıcı alınamadı / failed to get active provider: %v\n", err)
		hintIfUnauthorized(err)
		return 1
	}
	if strings.TrimSpace(name) == "" {
		fmt.Println("Aktif harici sağlayıcı yok (yerel model kullanılıyor). / No active external provider (routing to the local model).")
		return 0
	}
	fmt.Println(name)
	return 0
}
