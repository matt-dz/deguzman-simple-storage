package main

import (
	"dss/internal/api"
	"dss/internal/logger"
	"dss/internal/middleware"
	"net/http"
)

var logging = logger.GetLogger()

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{key}", middleware.Chain(
		api.HandleGetFile,
		middleware.Authenticate(),
	))

	logging.Info("Starting server on :80")
	server := &http.Server{
		Addr:    ":80",
		Handler: mux,
	}

	if err := server.ListenAndServe(); err != nil {
		panic(err)
	}

}
