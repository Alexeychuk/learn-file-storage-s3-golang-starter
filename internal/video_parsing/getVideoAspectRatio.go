package videoparsing

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"os/exec"
)

type ffprobeResult struct {
	Streams []struct {
		Width  float64 `json:"width"` // Capitalize field names
		Height float64 `json:"height"`
	} `json:"streams"`
}

// Helper function to compare floats with tolerance
func almostEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) <= tolerance
}

func GetVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	var buffer bytes.Buffer
	cmd.Stdout = &buffer
	err := cmd.Run()
	if err != nil {
		return "", err
	}

	var parsed_json ffprobeResult

	err = json.Unmarshal(buffer.Bytes(), &parsed_json)
	if err != nil {
		return "", err
	}

	if len(parsed_json.Streams) == 0 {
		return "", err
	}

	metadata := parsed_json.Streams[0]

	width := metadata.Width
	height := metadata.Height

	if width == 0 || height == 0 {
		return "", errors.New("invalid dimensions")
	}

	landscapeRatio := 16.0 / 9.0
	portraitRatio := 9.0 / 16.0

	currentRatio := width / height
	tolerance := 0.1
	if almostEqual(currentRatio, landscapeRatio, tolerance) {
		return "landscape", nil
	} else if almostEqual(currentRatio, portraitRatio, tolerance) {
		return "portrait", nil
	} else {
		return "other", nil
	}
}
