package api

import (
	"bufio"
	"context"
	"dss/internal/database"
	"dss/internal/logger"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var logging = logger.GetLogger()
var db = database.GetDb()
var ctx = context.Background()

const uploadLimit = 10 << 23 // 10 GB
const FileNameHeader = "X-File-Name"

var mountPath = os.Getenv("MOUNT_PATH")

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
	if mountPath == "" {
		logging.Error("Mount path not set")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	http.ServeFile(w, r, filepath.Join(mountPath, filePath))
	logging.Info("Successfully served file")
}

func HandleUploadFile(w http.ResponseWriter, r *http.Request) {
	logging.Info("Uploading file")

	if r.Header.Get(FileNameHeader) == "" {
		logging.Error("File name not set")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, uploadLimit)

	if mountPath == "" {
		logging.Error("Mount path not set")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	path := filepath.Join(mountPath, r.Header.Get(FileNameHeader))

	/* Add file to DB */
	logging.Info("Adding file to DB")
	var key uuid.UUID
	var pgErr *pgconn.PgError
	query := `
	INSERT INTO files (file_path) VALUES ($1)
	RETURNING key
	`
	if err := db.QueryRow(ctx, query, r.Header.Get(FileNameHeader)).Scan(&key); err != nil {
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			logging.Error("File already exists")
			http.Error(w, http.StatusText(http.StatusConflict), http.StatusConflict)
		} else {
			logging.Error("Failed to insert file into db", "error", err.Error())
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	logging.Info("Creating file", "path", path)
	file, err := os.Create(path)
	if err != nil {
		logging.Error("Failed to create file", "error", err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	reader := bufio.NewReader(r.Body)
	writer := bufio.NewWriter(file)
	var maxBytesError *http.MaxBytesError

	logging.Info("Writing file")
	for {
		buf := make([]byte, 1024)
		n, err := reader.Read(buf)
		if errors.Is(err, io.EOF) {
			break
		} else if errors.As(err, &maxBytesError) {
			logging.Error("File too large")
			http.Error(w, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
			return
		} else if err != nil {
			logging.Error("Failed to read request body", "error", err.Error())
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		if _, err := writer.Write(buf[:n]); err != nil {
			logging.Error("Failed to write to file", "error", err.Error())
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}
	logging.Info("Successfully wrote file")

	/* Encoding response */
	logging.Info("Encoding response")
	err = json.NewEncoder(w).Encode(UploadFileResponse{Key: key})
	if err != nil {
		logging.Error("Failed to encode response", "error", err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logging.Info("Successfully encoded response")
}

func HandleListFiles(w http.ResponseWriter, r *http.Request) {
	logging.Info("Retrieving files")
	q := r.URL.Query()

	/* Parsing query parameters */
	logging.Info("Parsing query parameters")

	limit := 10 // Default limit
	var err error
	if limit_str := q.Get("limit"); limit_str != "" {
		if limit, err = strconv.Atoi(limit_str); err != nil {
			logging.Error("Failed to parse limit", "error", err.Error())
			http.Error(w, "Invalid limit value", http.StatusBadRequest)
			return
		}
		if limit > 100 {
			logging.Error("Limit too large")
			http.Error(w, "Limit too large", http.StatusBadRequest)
			return
		}
	}

	var sortBy string
	if sortBy = q.Get("sort_by"); sortBy != "" {
		switch strings.ToLower(sortBy) {
		case "created_at":
			sortBy = "created_at"
		case "updated_at":
			sortBy = "updated_at"
		case "random":
			sortBy = "RANDOM()"
		default:
			logging.Error("Invalid sort_by")
			http.Error(w, "Invalid sort_by value", http.StatusBadRequest)
			return
		}
	} else {
		sortBy = "created_at"
	}

	var order string
	if order = q.Get("order"); order != "" {
		switch strings.ToUpper(order) {
		case "ASC":
			order = "ASC"
		case "DESC":
			order = "DESC"
		default:
			logging.Error("Invalid order")
			http.Error(w, "Invalid order value", http.StatusBadRequest)
			return
		}
	} else {
		order = "DESC"
	}

	tags := q["tag"]
	logging.Info("Parsed query parameters", "limit", limit, "sort_by", sortBy, "order", order, "tags", tags)

	/* Building query */
	logging.Info("Querying DB")
	var rows pgx.Rows
	var keys []uuid.UUID = make([]uuid.UUID, 0)

	if len(tags) == 0 {
		query := fmt.Sprintf(`
		SELECT key FROM files
		ORDER BY %s %s
		LIMIT $1
		`, sortBy, order)

		rows, err = db.Query(ctx, query, limit)
		if err != nil {
			logging.Error("Failed to query db", "error", err.Error())
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	} else {
		query := fmt.Sprintf(`
		SELECT key FROM files
		WHERE tags @> $1
		ORDER BY %s %s
		LIMIT $2
		`, sortBy, order)

		rows, err = db.Query(ctx, query, tags, limit)
		if err != nil {
			logging.Error("Failed to query db", "error", err.Error())
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	logging.Info("Successfully queried db")

	/* Scan Rows */
	logging.Info("Scanning rows")
	defer rows.Close()
	for rows.Next() {
		var key uuid.UUID
		if err := rows.Scan(&key); err != nil {
			logging.Error("Failed to scan row", "error", err.Error())
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		keys = append(keys, key)
	}
	logging.Info("Successfully scanned rows")

	/* Encode response */
	logging.Info("Encoding response")
	err = json.NewEncoder(w).Encode(&ListFilesResponse{Keys: keys})
	if err != nil {
		logging.Error("Failed to encode response", "error", err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logging.Info("Successfully encoded response")
}
