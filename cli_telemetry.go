// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"memo/internal/replcli"
)

// runTelemetryCommand implements `memo telemetry <verb>` — the terminal side of
// Settings › Privacy: see what the optional anonymous daily summary would send,
// answer yes or no, give the install a new id, or delete what the server holds.
// Same shape as `memo agent` / `memo remote`. Nothing is ever sent without an
// explicit `on`.
func runTelemetryCommand(args []string) int {
	if len(args) < 1 {
		printTelemetryUsage()
		return 1
	}
	verb := args[0]

	fs := flag.NewFlagSet("telemetry "+verb, flag.ContinueOnError)
	fs.Usage = printTelemetryUsage
	port := fs.Int("port", 8090, "Backend port")
	token := fs.String("token", "", "Device or session token — required if the backend was started with --lan")
	raw := fs.Bool("raw", false, "preview: print the exact bytes that would be sent, not an indented copy")
	flagArgs, _ := splitFlagsAndPositional(args[1:], nil)
	if err := fs.Parse(flagArgs); err != nil {
		return 1
	}

	client := replcli.NewClient(fmt.Sprintf("http://127.0.0.1:%d", *port))
	if *token != "" {
		client.SetToken(*token)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second) // reset/delete wait for the server
	defer cancel()

	switch verb {
	case "status":
		return telemetryStatusCmd(ctx, client)
	case "preview":
		return telemetryPreviewCmd(ctx, client, *raw)
	case "on":
		return telemetrySetCmd(ctx, client, true)
	case "off":
		return telemetrySetCmd(ctx, client, false)
	case "reset-id":
		return telemetryActionCmd(ctx, "reset-id", client.ResetTelemetryID,
			"✓ Yeni rastgele kimlik verildi; eskisinin sunucudaki kayıtları silinmek üzere istendi / a new random id was set; the server was asked to forget the old one")
	case "delete":
		return telemetryActionCmd(ctx, "delete", client.DeleteTelemetryData,
			"✓ Sunucudaki verilerin silinmesi istendi ve paylaşım kapatıldı / the server was asked to delete your data and sharing is off")
	default:
		printTelemetryUsage()
		return 1
	}
}

func printTelemetryUsage() {
	fmt.Fprintln(os.Stderr, `kullanım / usage:
  memo telemetry status            ne durumda / where things stand
  memo telemetry preview [--raw]   bir sonraki özetin TAM içeriği / the exact content of the next summary
  memo telemetry on                günlük anonim özeti paylaş / share the anonymous daily summary
  memo telemetry off               paylaşma / do not share
  memo telemetry reset-id          yeni rastgele kimlik / a new random id
  memo telemetry delete            sunucudaki verini sil ve paylaşımı kapat / delete your data on the server and stop sharing

her komut / every command: [--port N] [--token T]
  MEMO_TELEMETRY=off ortam değişkeni her cevabı geçersiz kılar ve hiçbir ağ isteği yapılmaz.
  the MEMO_TELEMETRY=off environment variable overrides any answer and stops all network use.
  Ayrıntı / details: docs/TELEMETRY.md`)
}

func telemetryAnswer(consent string) string {
	switch consent {
	case "yes":
		return "evet, paylaşıyor / yes, sharing"
	case "no":
		return "hayır, paylaşmıyor / no, not sharing"
	}
	return "cevap yok (kaynak derleme), hiçbir şey gönderilmiyor / no answer (source build), nothing is sent"
}

func telemetryStatusCmd(ctx context.Context, c *replcli.Client) int {
	st, err := c.GetTelemetryStatus(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "durum alınamadı / failed to get status: %v\n", err)
		hintIfUnauthorized(err)
		return 1
	}
	fmt.Printf("Cevap / answer:      %s\n", telemetryAnswer(st.Consent))
	if st.EnvOff {
		fmt.Println("MEMO_TELEMETRY=off:  AÇIK — hiçbir şey sayılmaz ya da gönderilmez / ON — nothing is counted or sent")
	}
	fmt.Printf("Sürüm türü / build:  %s\n", st.Build)
	fmt.Printf("Adres / endpoint:    %s\n", st.Endpoint)
	fmt.Printf("Cihaz kimliği / id:  %s… (rastgele / random)\n", st.DeviceID)
	if st.ConsentDay != "" {
		fmt.Printf("Cevap günü / answered on: %s (UTC)\n", st.ConsentDay)
	}
	if st.LastSentDay != "" {
		fmt.Printf("Son gönderilen gün / last day sent: %s\n", st.LastSentDay)
	}
	return 0
}

func telemetryPreviewCmd(ctx context.Context, c *replcli.Client, raw bool) int {
	p, err := c.GetTelemetryPreview(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "önizleme alınamadı / failed to get the preview: %v\n", err)
		hintIfUnauthorized(err)
		return 1
	}
	if p.Empty {
		fmt.Println("Gönderilecek bir şey yok: son günlerde hiç kullanım kaydı yok / nothing to send: no activity on recent days")
		return 0
	}
	if raw {
		fmt.Println(p.JSON) // the exact bytes
		return 0
	}
	var out bytes.Buffer
	if err := json.Indent(&out, []byte(p.JSON), "", "  "); err != nil {
		fmt.Println(p.JSON)
		return 0
	}
	fmt.Fprintf(os.Stderr, "# %s günü için gönderilecek özet (içerik değişmez, yalnız girintili) / the summary for %s (same content, indented)\n", p.Day, p.Day)
	fmt.Println(out.String())
	return 0
}

func telemetrySetCmd(ctx context.Context, c *replcli.Client, share bool) int {
	st, err := c.SetTelemetryConsent(ctx, share)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cevap kaydedilemedi / could not save the answer: %v\n", err)
		hintIfUnauthorized(err)
		return 1
	}
	if share {
		fmt.Println("✓ Evet: yarından itibaren günde en çok bir anonim özet gönderilir / yes: from tomorrow on, one anonymous summary a day at most")
	} else {
		fmt.Println("✓ Hayır: hiçbir şey gönderilmez / no: nothing is sent")
	}
	if st.EnvOff {
		fmt.Println("(MEMO_TELEMETRY=off açık olduğu için bu cevap şimdilik etkisiz / ignored for now because MEMO_TELEMETRY=off is set)")
	}
	return 0
}

func telemetryActionCmd(ctx context.Context, name string, do func(context.Context) error, ok string) int {
	if err := do(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "%s yapılamadı / %s failed: %v\n", name, name, err)
		hintIfUnauthorized(err)
		return 1
	}
	fmt.Println(ok)
	return 0
}
