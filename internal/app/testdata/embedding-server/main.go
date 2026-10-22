// A real subprocess for embedding lifecycle tests; accepts llama-server flags.
package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
)

func main() {
	port, model := "", ""
	for i, arg := range os.Args {
		if i+1 < len(os.Args) {
			switch arg {
			case "--port":
				port = os.Args[i+1]
			case "--model":
				model = os.Args[i+1]
			}
		}
	}
	f, err := os.OpenFile(model+".starts", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	fmt.Fprintln(f, os.Getpid())
	f.Close()
	http.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(model, "loading") {
			w.WriteHeader(503)
			return
		}
		fmt.Fprint(w, `{"data":[{"id":"embedding-test"}]}`)
	})
	http.HandleFunc("/v1/embeddings", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[{"embedding":[1,0,0]}]}`)
	})
	http.HandleFunc("/crash", func(w http.ResponseWriter, r *http.Request) { os.Exit(2) })
	if err := http.ListenAndServe("127.0.0.1:"+port, nil); err != nil {
		panic(err)
	}
}
