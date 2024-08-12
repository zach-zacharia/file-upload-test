package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// FileType represents each file type with an extension and content description
type FileType struct {
	Extension string `json:"extension"`
	Content   string `json:"content"`
	Strings   string `json:"strings"`
}

// FileTypes represents the collection of file types
type FileTypes struct {
	FileTypes []FileType `json:"file_types"`
}

func main() {
	server := gin.Default()

	server.LoadHTMLGlob("./static/*.html")

	server.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	server.POST("/upload", func(c *gin.Context) {
		file, fileHeader, err := c.Request.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		fileExt := filepath.Ext(fileHeader.Filename)
		fileinfo, err := extensionCheck()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// 1st check
		contentDescription, ok := fileinfo[fileExt]
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "File extension is not allowed"})
			return
		}

		tempFile, err := os.CreateTemp("./user_files", fileHeader.Filename)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer os.Remove(tempFile.Name())
		defer tempFile.Close()

		_, err = io.Copy(tempFile, file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// 2nd check
		cmd := exec.Command("file", tempFile.Name())
		output, err := cmd.Output()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if !containsContent(string(output), contentDescription) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "File content does not match the expected type"})
			return
		}

		// Move the file to the final destination
		finalPath := "./user_files/" + fileHeader.Filename
		err = os.Rename(tempFile.Name(), finalPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "File uploaded successfully"})
	})

	server.Run(":2000")
}

// extensionCheck reads the JSON file and returns a map of file extensions to content descriptions
func extensionCheck() (map[string]string, error) {
	data, err := os.ReadFile("allowed.json")
	if err != nil {
		return nil, fmt.Errorf("error reading file: %v", err)
	}

	var fileTypes FileTypes
	err = json.Unmarshal(data, &fileTypes)
	if err != nil {
		return nil, fmt.Errorf("error parsing JSON file: %v", err)
	}

	extMap := make(map[string]string)
	for _, ft := range fileTypes.FileTypes {
		extMap[ft.Extension] = ft.Content
	}

	return extMap, nil
}

func containsContent(output, contentDescription string) bool {
	fmt.Printf("Command output: %s\n", output)                           // Debug print
	fmt.Printf("Expected content description: %s\n", contentDescription) // Debug print

	normalizedContent := normalizeContentDescription(contentDescription)
	return strings.Contains(strings.ToLower(output), normalizedContent)
}

func normalizeContentDescription(content string) string {
	return strings.ToLower(strings.TrimSpace(content))
}
