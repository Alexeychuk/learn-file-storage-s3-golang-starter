package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	videoparsing "github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/video_parsing"

	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const maxMemory = 1 << 30
	r.Body = http.MaxBytesReader(w, r.Body, maxMemory)

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

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error parsing file", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Not your video", err)
		return
	}

	fmt.Println("uploading video", videoID, "by user", userID)

	r.ParseMultipartForm(maxMemory)

	file, header, err := r.FormFile("video")
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
	if mtype != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Wrong format", err)
		return
	}

	exts, err := mime.ExtensionsByType(mediaType)
	if err != nil || len(exts) == 0 {
		respondWithError(w, http.StatusInternalServerError, "Error parsing extention", err)
		return
	}
	fileExt := strings.TrimPrefix(exts[0], ".")

	temp_file, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error crearting temp", err)
		return
	}
	defer os.Remove(temp_file.Name())
	defer temp_file.Close()

	_, err = io.Copy(temp_file, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error crearting temp", err)
		return
	}

	videoAspect, err := videoparsing.GetVideoAspectRatio(temp_file.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error aspect ratio", err)
		return
	}

	_, err = temp_file.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error crearting temp", err)
		return
	}

	randBytes := make([]byte, 32)
	_, err = rand.Read(randBytes)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error parsing file", err)
		return
	}

	processed_video_filepath, err := videoparsing.ProcessVideoForFastStart(temp_file.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error crearting processed video", err)
		return
	}

	processed_file, err := os.Open(processed_video_filepath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error opening processed video", err)
		return
	}
	defer processed_file.Close()
	defer os.Remove(processed_video_filepath) // Clean up processed file

	encoded := hex.EncodeToString(randBytes)

	video_key := fmt.Sprintf("%s/%s.%s", videoAspect, encoded, fileExt)

	obj_input := s3.PutObjectInput{Bucket: &cfg.s3Bucket, Key: &video_key, ContentType: &mtype, Body: processed_file}

	_, err = cfg.s3Client.PutObject(r.Context(), &obj_input)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error s3 save file", err)
		return
	}

	video_url := fmt.Sprintf("https://%s/%s", cfg.s3CfDistribution, video_key)
	video.VideoURL = &video_url

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error writing url to db", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
