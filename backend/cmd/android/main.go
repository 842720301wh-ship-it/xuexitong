//go:build android && cgo

package main

/*
#include <stdlib.h>
*/
import "C"

import (
	"embed"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"tiantianxiangshang/internal/server"
	"tiantianxiangshang/internal/settings"
)

// Only generated frontend assets are embedded, never the desktop data directory.
//
//go:embed all:dist
var assets embed.FS
var once sync.Once
var port C.int

//export StartTiantian
func StartTiantian(directory *C.char, token *C.char) C.int {
	once.Do(func() {
		root := C.GoString(directory)
		temp := filepath.Join(root, "tmp")
		if err := os.MkdirAll(temp, 0700); err != nil {
			log.Print("Cannot create application temp directory")
			return
		}
		if err := os.Setenv("TMPDIR", temp); err != nil {
			return
		}
		store, err := settings.Open(root)
		if err != nil {
			log.Print("Cannot open application settings")
			return
		}
		web, err := fs.Sub(assets, "dist")
		if err != nil {
			return
		}
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			log.Print("Cannot start local service")
			return
		}
		app := server.New(store)
		app.NativeToken = C.GoString(token)
		service := &http.Server{
			Handler:           app.HandlerFS(web),
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       60 * time.Second,
		}
		port = C.int(listener.Addr().(*net.TCPAddr).Port)
		go func() {
			if err := service.Serve(listener); err != nil && err != http.ErrServerClosed {
				log.Print("Local service stopped")
			}
		}()
	})
	return port
}

func main() {}
