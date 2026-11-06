// SPDX-License-Identifier: AGPL-3.0-or-later

package mail

import (
	"context"
	"encoding/xml"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ISPDBBaseURL is the Mozilla Thunderbird Autoconfig XML endpoint.
// Overridable in tests.
var ISPDBBaseURL = "https://autoconfig.thunderbird.net/v1.1/"

var presetConfigs = map[string]ServerConfig{
	"gmail.com": {
		Provider: ProviderGoogle,
		IMAPHost: "imap.gmail.com",
		IMAPPort: 993,
		SMTPHost: "smtp.gmail.com",
		SMTPPort: 587,
		AuthType: AuthOAuth2,
		Source:   "preset",
	},
	"googlemail.com": {
		Provider: ProviderGoogle,
		IMAPHost: "imap.gmail.com",
		IMAPPort: 993,
		SMTPHost: "smtp.gmail.com",
		SMTPPort: 587,
		AuthType: AuthOAuth2,
		Source:   "preset",
	},
	"outlook.com": {
		Provider: ProviderMicrosoft,
		IMAPHost: "outlook.office365.com",
		IMAPPort: 993,
		SMTPHost: "smtp.office365.com",
		SMTPPort: 587,
		AuthType: AuthOAuth2,
		Source:   "preset",
	},
	"outlook.com.tr": {
		Provider: ProviderMicrosoft,
		IMAPHost: "outlook.office365.com",
		IMAPPort: 993,
		SMTPHost: "smtp.office365.com",
		SMTPPort: 587,
		AuthType: AuthOAuth2,
		Source:   "preset",
	},
	"hotmail.com": {
		Provider: ProviderMicrosoft,
		IMAPHost: "outlook.office365.com",
		IMAPPort: 993,
		SMTPHost: "smtp.office365.com",
		SMTPPort: 587,
		AuthType: AuthOAuth2,
		Source:   "preset",
	},
	"hotmail.com.tr": {
		Provider: ProviderMicrosoft,
		IMAPHost: "outlook.office365.com",
		IMAPPort: 993,
		SMTPHost: "smtp.office365.com",
		SMTPPort: 587,
		AuthType: AuthOAuth2,
		Source:   "preset",
	},
	"live.com": {
		Provider: ProviderMicrosoft,
		IMAPHost: "outlook.office365.com",
		IMAPPort: 993,
		SMTPHost: "smtp.office365.com",
		SMTPPort: 587,
		AuthType: AuthOAuth2,
		Source:   "preset",
	},
	"msn.com": {
		Provider: ProviderMicrosoft,
		IMAPHost: "outlook.office365.com",
		IMAPPort: 993,
		SMTPHost: "smtp.office365.com",
		SMTPPort: 587,
		AuthType: AuthOAuth2,
		Source:   "preset",
	},
	"office365.com": {
		Provider: ProviderMicrosoft,
		IMAPHost: "outlook.office365.com",
		IMAPPort: 993,
		SMTPHost: "smtp.office365.com",
		SMTPPort: 587,
		AuthType: AuthOAuth2,
		Source:   "preset",
	},
	"yahoo.com": {
		Provider: ProviderYahoo,
		IMAPHost: "imap.mail.yahoo.com",
		IMAPPort: 993,
		SMTPHost: "smtp.mail.yahoo.com",
		SMTPPort: 587,
		AuthType: AuthPassword,
		Source:   "preset",
	},
	"icloud.com": {
		Provider: ProviderCustom,
		IMAPHost: "imap.mail.me.com",
		IMAPPort: 993,
		SMTPHost: "smtp.mail.me.com",
		SMTPPort: 587,
		AuthType: AuthPassword,
		Source:   "preset",
	},
	"me.com": {
		Provider: ProviderCustom,
		IMAPHost: "imap.mail.me.com",
		IMAPPort: 993,
		SMTPHost: "smtp.mail.me.com",
		SMTPPort: 587,
		AuthType: AuthPassword,
		Source:   "preset",
	},
	"yandex.com": {
		Provider: ProviderCustom,
		IMAPHost: "imap.yandex.com",
		IMAPPort: 993,
		SMTPHost: "smtp.yandex.com",
		SMTPPort: 465,
		AuthType: AuthPassword,
		Source:   "preset",
	},
	"yandex.com.tr": {
		Provider: ProviderCustom,
		IMAPHost: "imap.yandex.com",
		IMAPPort: 993,
		SMTPHost: "smtp.yandex.com",
		SMTPPort: 465,
		AuthType: AuthPassword,
		Source:   "preset",
	},
}

// PresetForProvider returns the default server config for a known provider.
func PresetForProvider(p ProviderType) ServerConfig {
	switch p {
	case ProviderGoogle:
		return presetConfigs["gmail.com"]
	case ProviderMicrosoft:
		return presetConfigs["outlook.com"]
	case ProviderYahoo:
		return presetConfigs["yahoo.com"]
	default:
		return ServerConfig{Provider: ProviderCustom, IMAPPort: 993, SMTPPort: 587, AuthType: AuthPassword, Source: "manual"}
	}
}

type ispdbClientConfig struct {
	XMLName       xml.Name          `xml:"clientConfig"`
	EmailProvider ispdbEmailSection `xml:"emailProvider"`
}

type ispdbEmailSection struct {
	ID              string            `xml:"id,attr"`
	IncomingServers []ispdbServerSpec `xml:"incomingServer"`
	OutgoingServers []ispdbServerSpec `xml:"outgoingServer"`
}

type ispdbServerSpec struct {
	Type           string   `xml:"type,attr"`
	Hostname       string   `xml:"hostname"`
	Port           string   `xml:"port"`
	SocketType     string   `xml:"socketType"`
	Authentication []string `xml:"authentication"`
}

// Autodiscover resolves IMAP/SMTP endpoints and OAuth2 support from an email address
// without asking the user for ports or hostnames.
func Autodiscover(ctx context.Context, email string) (ServerConfig, error) {
	parts := strings.Split(strings.TrimSpace(strings.ToLower(email)), "@")
	if len(parts) != 2 || parts[1] == "" {
		return ServerConfig{}, fmt.Errorf("invalid email address: %s", email)
	}
	domain := parts[1]

	// 1. Instant built-in presets
	if cfg, ok := presetConfigs[domain]; ok {
		return cfg, nil
	}

	// 2. Mozilla Thunderbird ISPDB (autoconfig.thunderbird.net)
	if cfg, err := queryISPDB(ctx, domain); err == nil && cfg.IMAPHost != "" {
		return cfg, nil
	}

	// 3. DNS MX inspection (detect Google Workspace or Microsoft 365 custom domains)
	if mxs, err := net.DefaultResolver.LookupMX(ctx, domain); err == nil {
		for _, mx := range mxs {
			host := strings.ToLower(mx.Host)
			if strings.Contains(host, "google.com") || strings.Contains(host, "googlemail.com") {
				cfg := presetConfigs["gmail.com"]
				cfg.Source = "mx-google"
				return cfg, nil
			}
			if strings.Contains(host, "outlook.com") || strings.Contains(host, "protection.outlook.com") {
				cfg := presetConfigs["outlook.com"]
				cfg.Source = "mx-microsoft"
				return cfg, nil
			}
			if strings.Contains(host, "yandex.") {
				cfg := presetConfigs["yandex.com"]
				cfg.Source = "mx-yandex"
				return cfg, nil
			}
		}
	}

	// 4. Conventional heuristic fallback
	return ServerConfig{
		Provider: ProviderCustom,
		IMAPHost: "imap." + domain,
		IMAPPort: 993,
		SMTPHost: "smtp." + domain,
		SMTPPort: 587,
		AuthType: AuthPassword,
		Source:   "heuristic",
	}, nil
}

func queryISPDB(ctx context.Context, domain string) (ServerConfig, error) {
	url := strings.TrimRight(ISPDBBaseURL, "/") + "/" + domain
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ServerConfig{}, err
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ServerConfig{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ServerConfig{}, fmt.Errorf("ispdb status %d", resp.StatusCode)
	}

	var parsed ispdbClientConfig
	if err := xml.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return ServerConfig{}, err
	}

	cfg := ServerConfig{
		Provider: ProviderCustom,
		IMAPPort: 993,
		SMTPPort: 587,
		AuthType: AuthPassword,
		Source:   "ispdb",
	}

	for _, in := range parsed.EmailProvider.IncomingServers {
		if strings.EqualFold(in.Type, "imap") && cfg.IMAPHost == "" {
			cfg.IMAPHost = strings.TrimSpace(in.Hostname)
			if p, err := strconv.Atoi(strings.TrimSpace(in.Port)); err == nil && p > 0 {
				cfg.IMAPPort = p
			}
			for _, a := range in.Authentication {
				if strings.EqualFold(strings.TrimSpace(a), "OAuth2") {
					cfg.AuthType = AuthOAuth2
				}
			}
		}
	}
	for _, out := range parsed.EmailProvider.OutgoingServers {
		if strings.EqualFold(out.Type, "smtp") && cfg.SMTPHost == "" {
			cfg.SMTPHost = strings.TrimSpace(out.Hostname)
			if p, err := strconv.Atoi(strings.TrimSpace(out.Port)); err == nil && p > 0 {
				cfg.SMTPPort = p
			}
		}
	}

	if strings.Contains(cfg.IMAPHost, "gmail.com") || strings.Contains(cfg.IMAPHost, "googlemail.com") {
		cfg.Provider = ProviderGoogle
		cfg.AuthType = AuthOAuth2
	} else if strings.Contains(cfg.IMAPHost, "office365.com") || strings.Contains(cfg.IMAPHost, "outlook.com") {
		cfg.Provider = ProviderMicrosoft
		cfg.AuthType = AuthOAuth2
	}

	return cfg, nil
}
