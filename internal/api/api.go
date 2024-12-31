package api

import (
	"context"
	"dss/internal/database"
	"dss/internal/logger"
	"errors"
	"github.com/jackc/pgx/v5"
	"net/http"
	"os"
	"path/filepath"
)

var logging = logger.GetLogger()
var db = database.GetDb()
var ctx = context.Background()

func HandleGetFile(w http.ResponseWriter, r *http.Request) {
	logging.Info("Retrieving file")

	/* Retrieving key from URL */
	logging.Info("Retrieving key from URL")
	key := r.PathValue("key")
	if key == "" {
		logging.Error("Key not set")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	/* Checking DB for file path */
	logging.Info("Checking DB for file")
	var filePath string
	query := `
	SELECT file_path FROM files WHERE key = $1
	`
	if err := db.QueryRow(ctx, query, key).Scan(&filePath); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logging.Error("File not found")
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		} else {
			logging.Error("Failed to query db", "error", err.Error())
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	/* Serving file */
	logging.Info("Serving file")
	mountPath := os.Getenv("MOUNT_PATH")
	if mountPath == "" {
		logging.Error("Mount path not set")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	http.ServeFile(w, r, filepath.Join(mountPath, filePath))
	logging.Info("Successfully served file")
}
