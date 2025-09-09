package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	const maxMemory = 10 << 20
	r.ParseMultipartForm(maxMemory)

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error parsing file", err)
		return
	}
	defer file.Close()

	mediaType := header.Header.Get("Content-Type")

	mtype, _, err := mime.ParseMediaType(mediaType)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error parsing file", err)
		return
	}
	if mtype != "image/jpeg" && mtype != "image/png" {
		respondWithError(w, http.StatusBadRequest, "Wrong format", err)
		return
	}

	exts, err := mime.ExtensionsByType(mediaType)
	if err != nil || len(exts) == 0 {
		respondWithError(w, http.StatusInternalServerError, "Error parsing extention", err)
		return
	}
	fileExt := strings.TrimPrefix(exts[0], ".")

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error parsing file", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Not your video", err)
		return
	}

	randBytes := make([]byte, 32)
	_, err = rand.Read(randBytes)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error parsing file", err)
		return
	}

	encoded := base64.RawStdEncoding.EncodeToString(randBytes)

	thumbnail_filepath := filepath.Join(cfg.assetsRoot, fmt.Sprintf("%s.%s", encoded, fileExt))

	fmt.Printf("%s\n", thumbnail_filepath)

	thumbnail_file, err := os.Create(thumbnail_filepath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating file", err)
		return
	}
	defer thumbnail_file.Close()

	_, err = io.Copy(thumbnail_file, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error parsing file", err)
		return
	}
	thumb_link := fmt.Sprintf("http://localhost:%s/assets/%s.%s", cfg.port, encoded, fileExt)
	video.ThumbnailURL = &thumb_link

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error parsing file", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
