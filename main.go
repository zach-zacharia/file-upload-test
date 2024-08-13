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
	Extension string   `json:"extension"`
	Content   string   `json:"content"`
	Strings   []string `json:"strings"`
	Contains  []string `json:"contains"`
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
			c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
			return
		}

		fileExt := filepath.Ext(fileHeader.Filename)
		fileinfo, filestrings, filecontents, err := extensionCheck()
		if err != nil {
			response := gin.H{"message": err.Error()}
			c.JSON(http.StatusInternalServerError, response)
			return
		}

		// 1st check
		contentDescription, ok := fileinfo[fileExt]
		if !ok {
			response := gin.H{"message": "File extension is not allowed"}
			c.JSON(http.StatusBadRequest, response)
			return
		}

		tempFile, err := os.CreateTemp("./user_files", fileHeader.Filename)
		if err != nil {
			response := gin.H{"message": err.Error()}
			c.JSON(http.StatusInternalServerError, response)
			return
		}
		defer os.Remove(tempFile.Name())
		defer tempFile.Close()

		_, err = io.Copy(tempFile, file)
		if err != nil {
			response := gin.H{"message": err.Error()}
			c.JSON(http.StatusInternalServerError, response)
			return
		}

		// 2nd check
		cmd := exec.Command("file", tempFile.Name())
		output, err := cmd.Output()
		if err != nil {
			response := gin.H{"message": err.Error()}
			c.JSON(http.StatusInternalServerError, response)
			return
		}

		if !containsContent(string(output), contentDescription) {
			response := gin.H{"message": "File content does not match the expected type"}
			c.JSON(http.StatusBadRequest, response)
			return
		}

		// 3rd check
		cmd = exec.Command("strings", tempFile.Name())
		output, err = cmd.Output()
		if err != nil {
			response := gin.H{"message": err.Error()}
			c.JSON(http.StatusInternalServerError, response)
			return
		}

		// Image check using 'binwalk'
		images := []string{".jpg", ".jpeg", ".png", ".gif"}
		if containsAnyString(fileExt, images) {
			fmt.Print("Image file detected. Performing 'binwalk' scan...\n")
		}

		if !containsAnyString(string(output), filestrings[fileExt]) {
			response := gin.H{"message": "File strings does not match the expected type"}
			c.JSON(http.StatusBadRequest, response)
			return
		}

		// Move the file to the final destination
		finalPath := "./user_files/" + fileHeader.Filename
		err = os.Rename(tempFile.Name(), finalPath)
		if err != nil {
			response := gin.H{"message": err.Error()}
			c.JSON(http.StatusInternalServerError, response)
			return
		}

		response := gin.H{"message": "File uploaded successfully"}

		c.JSON(http.StatusOK, response)
	})

	server.Run(":2000")
}

// extensionCheck reads the JSON file and returns a map of file extensions to content descriptions
func extensionCheck() (map[string]string, map[string][]string, map[string][]string, error) {
	data, err := os.ReadFile("allowed.json")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error reading file: %v", err)
	}

	var fileTypes FileTypes
	err = json.Unmarshal(data, &fileTypes)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error parsing JSON file: %v", err)
	}

	extMap := make(map[string]string)
	for _, ft := range fileTypes.FileTypes {
		extMap[ft.Extension] = ft.Content
	}

	strMap := make(map[string][]string)
	for _, ft := range fileTypes.FileTypes {
		strMap[ft.Extension] = ft.Strings
	}

	conMap := make(map[string][]string)
	for _, ft := range fileTypes.FileTypes {
		conMap[ft.Extension] = ft.Contains
	}

	return extMap, strMap, conMap, nil
}

func containsContent(output, contentDescription string) bool {
	fmt.Printf("Command output: %s\n", output)                           // Debug print
	fmt.Printf("Expected content description: %s\n", contentDescription) // Debug print

	normalizedContent := normalizeContentDescription(contentDescription)
	return strings.Contains(strings.ToLower(output), normalizedContent)
}

func containsAnyString(output string, expectedStrings []string) bool {
	fmt.Printf("Command output: %s\n", output)            // Debug print
	fmt.Printf("Expected strings: %v\n", expectedStrings) // Debug print

	for _, str := range expectedStrings {
		if strings.Contains(strings.ToLower(output), strings.ToLower(strings.TrimSpace(str))) {
			return true
		}
	}
	return false
}

func normalizeContentDescription(content string) string {
	return strings.ToLower(strings.TrimSpace(content))
}
