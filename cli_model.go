package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"memo/internal/replcli"
)

// runModelCommand implements `memo model <verb> [flags]` — CLI management of
// local chat/embedding models (search Hugging Face, download, start/stop),
// same shape as `memo remote`/`memo provider`/`memo agent`. Exists because
// setting up local memory/embedding on a headless self-hosted install
// otherwise required a raw curl against /api/models/search,
// /api/models/download, /api/models/download/progress and
// /api/models/embedding/start in sequence, with no CLI path at all.
func runModelCommand(args []string) int {
	if len(args) < 1 {
		printModelUsage()
		return 1
	}
	verb := args[0]

	fs := flag.NewFlagSet("model "+verb, flag.ContinueOnError)
	fs.Usage = printModelUsage
	port := fs.Int("port", 8090, "Backend port")
	token := fs.String("token", "", "Device or session token — required if the backend was started with --lan")
	size := fs.Int64("size", 0, "download: expected file size in bytes (memo model files <repo> reports it) — only used to show a percentage, the download itself works without it")
	ctxSize := fs.Int("ctx", 0, "start: context size, 0 = backend default")
	modelPort := fs.Int("model-port", 0, "start: port for the llama.cpp server, 0 = auto")
	gpuLayers := fs.Int("gpu", -1, "start/start-embedding: GPU layers to offload, -1 = auto")

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
	case "list":
		return modelListCmd(ctx, client)
	case "status":
		return modelStatusCmd(ctx, client)
	case "search":
		if len(positional) != 1 {
			fmt.Fprintln(os.Stderr, "kullanım / usage: memo model search <sorgu/query>")
			return 1
		}
		return modelSearchCmd(ctx, client, positional[0])
	case "files":
		if len(positional) != 1 {
			fmt.Fprintln(os.Stderr, "kullanım / usage: memo model files <repo/repo-id>")
			return 1
		}
		return modelFilesCmd(ctx, client, positional[0])
	case "download":
		if len(positional) != 2 {
			fmt.Fprintln(os.Stderr, "kullanım / usage: memo model download <repo> <dosya adı/filename> [--size N]")
			return 1
		}
		// Downloading can legitimately take minutes on a slow connection or a
		// large file — the short 10s ctx above is for the initial "queue it"
		// call only; a separate, much longer one guards the polling loop.
		dlCtx, dlCancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer dlCancel()
		return modelDownloadCmd(dlCtx, client, positional[0], positional[1], *size)
	case "start":
		if len(positional) != 1 {
			fmt.Fprintln(os.Stderr, "kullanım / usage: memo model start <path> [--ctx N] [--model-port N] [--gpu N]")
			return 1
		}
		return modelStartCmd(ctx, client, positional[0], *ctxSize, *modelPort, *gpuLayers)
	case "start-embedding":
		if len(positional) != 1 {
			fmt.Fprintln(os.Stderr, "kullanım / usage: memo model start-embedding <path> [--gpu N]")
			return 1
		}
		return modelStartEmbeddingCmd(ctx, client, positional[0], *gpuLayers)
	default:
		printModelUsage()
		return 1
	}
}

func printModelUsage() {
	fmt.Fprintln(os.Stderr, `kullanım / usage:
  memo model list
  memo model status
  memo model search <sorgu/query>
  memo model files <repo>
  memo model download <repo> <dosya/filename> [--size N]
  memo model start <path> [--ctx N] [--model-port N] [--gpu N]
  memo model start-embedding <path> [--gpu N]

her komut / every command: [--port N] [--token T]
  --lan ile başlatılmış bir backend her istekte kimlik ister — o zaman
  --token (mevcut bir cihaz/oturum token'ı) gerekir.
  a backend started with --lan requires a credential on every request —
  pass --token (an existing device/session token) in that case.

örnek akış / example flow:
  memo model search nomic-embed-text
  memo model files nomic-ai/nomic-embed-text-v1.5-GGUF
  memo model download nomic-ai/nomic-embed-text-v1.5-GGUF nomic-embed-text-v1.5.Q4_K_M.gguf --size 84106624
  memo model start-embedding ~/.memo/data/models/nomic-embed-text-v1.5.Q4_K_M.gguf`)
}

func modelListCmd(ctx context.Context, c *replcli.Client) int {
	models, err := c.ListLocalModels(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "modeller alınamadı / failed to list models: %v\n", err)
		hintIfUnauthorized(err)
		return 1
	}
	if len(models) == 0 {
		fmt.Println("İndirilmiş model yok. / No downloaded models.")
		return 0
	}
	for _, m := range models {
		kind := "chat"
		if m.IsEmbedding {
			kind = "embedding"
		}
		fmt.Printf("%s\t(%s, %.1f MB)\t%s\n", m.Filename, kind, float64(m.Size)/1024/1024, m.Path)
	}
	return 0
}

func modelStatusCmd(ctx context.Context, c *replcli.Client) int {
	chat, err := c.ModelStatus(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "durum alınamadı / failed to get status: %v\n", err)
		hintIfUnauthorized(err)
		return 1
	}
	if chat.Running {
		fmt.Printf("Chat model: çalışıyor / running (%s, port %d)\n", chat.ModelName, chat.Port)
	} else {
		fmt.Println("Chat model: çalışmıyor / not running")
	}
	emb, err := c.EmbeddingStatus(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "embedding durumu alınamadı / failed to get embedding status: %v\n", err)
		return 1
	}
	if emb.Running {
		fmt.Printf("Embedding: çalışıyor / running (%s, port %d)\n", emb.ModelName, emb.Port)
	} else {
		fmt.Println("Embedding: çalışmıyor / not running")
	}
	return 0
}

func modelSearchCmd(ctx context.Context, c *replcli.Client, query string) int {
	results, err := c.SearchModels(ctx, query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arama başarısız / search failed: %v\n", err)
		hintIfUnauthorized(err)
		return 1
	}
	if len(results) == 0 {
		fmt.Println("Sonuç yok. / No results.")
		return 0
	}
	for _, r := range results {
		fmt.Printf("%s\t(⬇ %d, ♥ %d)\n", r.ID, r.Downloads, r.Likes)
	}
	return 0
}

func modelFilesCmd(ctx context.Context, c *replcli.Client, repo string) int {
	files, err := c.ListModelFiles(ctx, repo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dosyalar alınamadı / failed to list files: %v\n", err)
		hintIfUnauthorized(err)
		return 1
	}
	if len(files) == 0 {
		fmt.Println("Bu repoda .gguf dosyası bulunamadı. / No .gguf files found in this repo.")
		return 0
	}
	for _, f := range files {
		fmt.Printf("%s\t%.1f MB\t(--size %d)\n", f.Filename, float64(f.Size)/1024/1024, f.Size)
	}
	return 0
}

func modelDownloadCmd(ctx context.Context, c *replcli.Client, repo, filename string, size int64) int {
	if err := c.DownloadModel(ctx, repo, filename, size); err != nil {
		fmt.Fprintf(os.Stderr, "indirme başlatılamadı / failed to start download: %v\n", err)
		hintIfUnauthorized(err)
		return 1
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	var lastPercent float64 = -1
	for {
		select {
		case <-ctx.Done():
			fmt.Fprintln(os.Stderr, "\nzaman aşımı / timed out waiting for the download to finish")
			return 1
		case <-ticker.C:
			progress, err := c.DownloadProgress(ctx)
			if err != nil {
				fmt.Fprintf(os.Stderr, "\nilerleme alınamadı / failed to get progress: %v\n", err)
				return 1
			}
			var current *replcli.ModelDownloadProgress
			for i := range progress {
				if progress[i].RepoID == repo && progress[i].Filename == filename {
					current = &progress[i]
					break
				}
			}
			if current == nil {
				// No longer in the active list — either it finished, or it
				// was never actually queued (already downloaded, bad repo).
				// Either way there's nothing left to poll.
				fmt.Println("\n✓ Bitti / done (ya da zaten indirilmişti / or it was already downloaded).")
				return 0
			}
			if current.Error != "" {
				fmt.Printf("\nindirme hatası / download error: %s\n", current.Error)
				return 1
			}
			if current.Percent != lastPercent {
				fmt.Printf("\r%.1f%% (%s)          ", current.Percent, current.Speed)
				lastPercent = current.Percent
			}
		}
	}
}

func modelStartCmd(ctx context.Context, c *replcli.Client, path string, ctxSize, port, gpuLayers int) int {
	fmt.Println("Model yükleniyor, bu biraz sürebilir... / loading the model, this can take a moment...")
	if err := c.StartModel(ctx, path, ctxSize, port, gpuLayers); err != nil {
		fmt.Fprintf(os.Stderr, "model başlatılamadı / failed to start the model: %v\n", err)
		hintIfUnauthorized(err)
		return 1
	}
	fmt.Println("✓ Model çalışıyor / model running.")
	return 0
}

func modelStartEmbeddingCmd(ctx context.Context, c *replcli.Client, path string, gpuLayers int) int {
	fmt.Println("Embedding modeli yükleniyor... / loading the embedding model...")
	if err := c.StartEmbedding(ctx, path, gpuLayers); err != nil {
		fmt.Fprintf(os.Stderr, "embedding başlatılamadı / failed to start embedding: %v\n", err)
		hintIfUnauthorized(err)
		return 1
	}
	fmt.Println("✓ Embedding çalışıyor / embedding running.")
	return 0
}
