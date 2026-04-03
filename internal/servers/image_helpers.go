package servers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
)

type Hit struct {
	ID    string  `json:"id"`
	Score float32 `json:"score"`
	Path  string  `json:"path"`
}

type PathReq struct {
	Path string `json:"path"`
}

func SearchByPath(baseURL, filePath string) ([]Hit, error) {
	reqBody, _ := json.Marshal(PathReq{Path: filePath})

	url := fmt.Sprintf("%s/search_cbf_path", baseURL)
	resp, err := http.Post(url, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var hits []Hit
	json.NewDecoder(resp.Body).Decode(&hits)
	return hits, nil
}

func SearchByFile(baseURL, localPath string) ([]Hit, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	buf := new(bytes.Buffer)
	writer := multipart.NewWriter(buf)
	fw, _ := writer.CreateFormFile("file", localPath)
	io.Copy(fw, file)
	writer.Close()

	url := fmt.Sprintf("%s/search_cbf", baseURL)
	req, _ := http.NewRequest("POST", url, buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var hits []Hit
	json.NewDecoder(resp.Body).Decode(&hits)
	return hits, nil
}

func requiresImageSearch(prompt string) bool {
	keywords := []string{"cbf", "similar image", "diffraction", "pattern", "image search"}
	p := strings.ToLower(prompt)
	for _, kw := range keywords {
		if strings.Contains(p, kw) {
			return true
		}
	}
	return false
}

func extractImagePath(prompt string) string {
	// very naive; you can improve with regex / LLM-assisted extraction
	parts := strings.Fields(prompt)
	for _, p := range parts {
		if strings.HasSuffix(p, ".cbf") {
			return p
		}
	}
	return ""
}

func buildImageContext(hits []Hit) string {
	var out strings.Builder
	out.WriteString("Found similar images:\n")
	for _, h := range hits {
		out.WriteString(fmt.Sprintf(
			"- ID=%s Score=%.4f Path=%s\n",
			h.ID, h.Score, h.Path,
		))
	}
	return out.String()
}
