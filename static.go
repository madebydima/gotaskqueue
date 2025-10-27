package gotaskqueue

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static/*
var staticFiles embed.FS

// StaticHandler возвращает обработчик для статических файлов
func StaticHandler() http.Handler {
	fs, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(fs))
}
