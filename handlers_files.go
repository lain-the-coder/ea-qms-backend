package main

import (
	"database/sql"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/lain-the-coder/ea-qms-backend/internal/database"
	"github.com/lain-the-coder/ea-qms-backend/internal/logging"
)

func (cfg *apiConfig) HandlerUploadFile(w http.ResponseWriter, r *http.Request, user database.User) {
	type FileUploadResponse struct {
		FileName    string    `json:"file_name"`
		FileSize    int64     `json:"file_size"`
		ContentType string    `json:"content_type"`
		UploadedOn  time.Time `json:"uploaded_on"`
	}
	log := logging.LoggerFrom(r.Context())
	// cap the body first, limit is in place when the reading starts
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes+1<<20)
	// extract and validate path parameters
	ccIDRawStr := r.PathValue("ccID")
	fieldName := r.PathValue("fieldName")
	ccID := strings.TrimSpace(ccIDRawStr)
	if ccID == "" {
		log.Warn("file upload failed", "reason", "CC-ID blank")
		respondWithError(w, "CC-ID cannot be blank", http.StatusBadRequest)
		return
	}
	if fieldName != fieldImplementationEvidence {
		log.Warn("file upload failed", "reason", "field name is not implementation_evidence in url path")
		respondWithError(w, "Field name in path parameter is not one of accepted values", http.StatusBadRequest)
		return
	}
	// parse the multipart body
	// keep up to 1 MB in RAM; anything larger spills to OS disk (/tmp)
	err := r.ParseMultipartForm(maxMultipartMemory)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			log.Warn("file upload failed", "reason", "Request body exceeded 11 MB limit")
			respondWithError(w, "File exceeds maximum allowed size of 10 MB", http.StatusBadRequest)
			return
		}
		log.Warn("file upload failed", "reason", "Invalid multipart form payload", "error", err)
		respondWithError(w, "Invalid multipart form payload", http.StatusBadRequest)
		return
	}
	// purge temporary disk files when handler finishes
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	// extract file stream and file metadata
	file, header, err := r.FormFile("file")
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			log.Warn("file upload failed", "reason", "Missing file payload in request")
			respondWithError(w, "Missing file payload in request", http.StatusBadRequest)
			return
		}
		log.Error("file upload failed", "reason", "Error retrieving file from request", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	// close the file descriptor to prevent resource leaks
	defer file.Close()
	// cheap rejections, before reading bytes
	if header.Size > maxUploadBytes {
		log.Warn("file upload failed", "reason", "File must be 10 MB or smaller")
		respondWithError(w, "File must be 10 MB or smaller", http.StatusBadRequest)
		return
	}
	if header.Filename == "" {
		log.Warn("file upload failed", "reason", "File name cannot be blank")
		respondWithError(w, "File name cannot be blank", http.StatusBadRequest)
		return
	}
	if strings.ToLower(filepath.Ext(header.Filename)) != ".pdf" {
		log.Warn("file upload failed", "reason", "Only PDF files are accepted")
		respondWithError(w, "Only PDF files are accepted", http.StatusBadRequest)
		return
	}
	// read file and verify
	data, err := io.ReadAll(file)
	if err != nil {
		// since MaxBytesReader already passed, any error here is a true OS/Disk failure
		log.Error("file upload failed", "reason", "Failed to read temp file from disk", "error", err)
		respondWithError(w, "Failed to read uploaded file contents", http.StatusInternalServerError)
		return
	}
	// ground-truth byte validations
	if len(data) == 0 {
		log.Warn("file upload failed", "reason", "Uploaded file is 0 bytes")
		respondWithError(w, "File payload cannot be empty", http.StatusBadRequest)
		return
	}
	if len(data) > maxUploadBytes {
		log.Warn("file upload failed", "reason", "Actual file size exceeds 10 MB limit")
		respondWithError(w, "File must be 10 MB or smaller", http.StatusBadRequest)
		return
	}
	// a renamed .exe is caught here
	if http.DetectContentType(data) != contentTypePDF {
		log.Warn("file upload failed", "reason", "Magic byte check failed - payload signature is not PDF")
		respondWithError(w, "Only PDF files are accepted", http.StatusBadRequest)
		return
	}
	safeFilename := sanitizeFilename(header.Filename)
	// open transaction
	tx, err := cfg.rawDB.BeginTx(r.Context(), nil)
	if err != nil {
		log.Error("file upload failed", "reason", "could not begin transaction", "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()
	qtx := cfg.db.WithTx(tx)
	// retrieve cc details with intent of updating
	cc, err := qtx.GetChangeControlForUpdate(r.Context(), ccID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Warn("file upload failed", "reason", "cc not found", "cc_id", ccID)
			respondWithError(w, "Change Control not found", http.StatusNotFound)
			return
		}
		log.Error("file upload failed", "reason", "cc lookup failed", "cc_id", ccID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	// ownership check
	if user.ID != cc.ChangeOwnerID {
		log.Warn("file upload failed", "reason", "user is not owner of cc", "cc_id", ccID)
		respondWithError(w, "Forbidden", http.StatusForbidden)
		return
	}
	// state check
	if cc.CurrentState != stateInImplementation {
		log.Warn("file upload failed", "reason",
			"File upload to Implementation Evidence field allowed only in In Implementation state",
			"cc_id", ccID)
		respondWithError(w,
			"File upload to Implementation Evidence field allowed only in In Implementation state",
			http.StatusConflict)
		return
	}
	// upsert file attachment
	fileAttachmentRow, err := qtx.UpsertFileAttachment(r.Context(), database.UpsertFileAttachmentParams{
		ChangeControlID: cc.ID,
		FieldName:       fieldImplementationEvidence,
		FileName:        safeFilename,
		FileSize:        int64(len(data)),
		ContentType:     contentTypePDF,
		FileData:        data,
		UploadedByID:    user.ID,
	})
	if err != nil {
		log.Error("file upload failed", "reason", "file attachment table upsert failed", "cc_id", ccID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	err = qtx.TouchChangeControl(r.Context(), database.TouchChangeControlParams{
		CcID:            ccID,
		LastUpdatedByID: user.ID,
	})
	if err != nil {
		log.Error("file upload failed", "reason", "cc update failed", "cc_id", ccID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	// commit
	err = tx.Commit()
	if err != nil {
		log.Error("file upload failed", "reason", "db commit failed", "cc_id", ccID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	// response
	log.Info("file uploaded", "cc_id", ccID, "field_name", fieldName,
		"file_name", safeFilename, "file_size", len(data))
	respondWithJSON(w, http.StatusOK, FileUploadResponse{
		FileName:    fileAttachmentRow.FileName,
		FileSize:    fileAttachmentRow.FileSize,
		ContentType: fileAttachmentRow.ContentType,
		UploadedOn:  fileAttachmentRow.UploadedOn,
	})
}

func (cfg *apiConfig) HandlerDownloadFile(w http.ResponseWriter, r *http.Request, user database.User) {
	log := logging.LoggerFrom(r.Context())
	// extract and validate path parameters
	ccIDRawStr := r.PathValue("ccID")
	fieldName := r.PathValue("fieldName")
	ccID := strings.TrimSpace(ccIDRawStr)
	if ccID == "" {
		log.Warn("file download failed", "reason", "CC-ID blank")
		respondWithError(w, "CC-ID cannot be blank", http.StatusBadRequest)
		return
	}
	if fieldName != fieldImplementationEvidence {
		log.Warn("file download failed", "reason", "field name is not implementation_evidence in url path")
		respondWithError(w, "Field name in path parameter is not one of accepted values", http.StatusBadRequest)
		return
	}
	// no qtx since both read only operations, atomicity unnecessary
	id, err := cfg.db.GetChangeControlIDByCcID(r.Context(), ccID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Warn("file download failed", "reason", "cc not found", "cc_id", ccID)
			respondWithError(w, "Change Control not found", http.StatusNotFound)
			return
		}
		log.Error("file download failed", "reason", "cc lookup failed", "cc_id", ccID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	row, err := cfg.db.GetFileAttachment(r.Context(), database.GetFileAttachmentParams{
		ChangeControlID: id,
		FieldName:       fieldImplementationEvidence,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Warn("file download failed", "reason", "no file uploaded for this field", "cc_id", ccID)
			respondWithError(w, "File not found for Implementation Evidence field", http.StatusNotFound)
			return
		}
		log.Error("file download failed", "reason", "file attachment lookup failed", "cc_id", ccID, "error", err)
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	// cannot use RespondWithJson function since here we're writing application/pdf
	// 3 header writes
	w.Header().Set("Content-Type", row.ContentType)
	// download rather than display and filename as set
	w.Header().Set("Content-Disposition", `attachment; filename="`+row.FileName+`"`)
	// lets the browser show a progress bar and detect a truncated transfer
	w.Header().Set("Content-Length", strconv.FormatInt(row.FileSize, 10))
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(row.FileData); err != nil {
		// headers and 200 OK have already gone out — the status cannot be changed.
		// The browser detects the truncation via Content-Length and surfaces it.
		log.Error("file download failed", "reason", "write to client failed",
			"cc_id", ccID, "error", err)
		return
	}
	log.Info("file downloaded", "cc_id", ccID, "field_name", fieldName,
		"file_name", row.FileName, "file_size", row.FileSize)
}
