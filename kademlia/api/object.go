package api

import (
	"crypto/sha1"
	"encoding/hex"
	"io"
	"net/http"
	"time"
)

type ObjectNode interface {
	HandleStore(key [20]byte, data []byte, ttl time.Duration) error
	LookupValue(hexKey string) (data []byte, ok bool)
}

func NewObjectsMux(n ObjectNode) http.Handler {
	mux := http.NewServeMux()

	// POST /objects
	mux.HandleFunc("POST /objects", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		// Read the whole body (consider a max size if you want)
		body, err := io.ReadAll(r.Body)
		if err != nil || len(body) <= 0 {
			http.Error(w, "empty or unreadable body", http.StatusBadRequest)
			return
		}
		sum := sha1.Sum(body)
		keyHex := hex.EncodeToString(sum[:])

		if err := n.HandleStore(sum, body, 0); err != nil {
			http.Error(w, "store failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Location", "/objects/"+keyHex)
		w.Header().Set("ETag", `"`+keyHex+`"`)

		ct := r.Header.Get("Content-Type")
		if ct == "" {
			ct = "application/octet-stream"
		}
		w.Header().Set("Content-Type", ct)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(body)
	})

	// GET /objects/{key}
	mux.HandleFunc("GET /objects/{hash}", func(w http.ResponseWriter, r *http.Request) {
		hash := r.PathValue("hash")
		if len(hash) != 40 {
			http.Error(w, "bad key (need 40 hex chars)", http.StatusBadRequest)
			return
		}

		if data, ok := n.LookupValue(hash); ok {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("ETag", `"`+hash+`"`)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	})
	return mux
}
