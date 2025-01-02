package api

import "github.com/google/uuid"

type UploadFileResponse struct {
	Key uuid.UUID `json:"key"`
}
