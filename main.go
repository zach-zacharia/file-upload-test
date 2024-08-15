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
		fileinfo, filestrings, err := extensionCheck()
		if err != nil {
			response := gin.H{"message": err.Error()}
			c.JSON(http.StatusInternalServerError, response)
			return
		}

		// Extension check
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

		// File description check
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

		// File strings check
		cmd = exec.Command("strings", tempFile.Name())
		output, err = cmd.Output()
		if err != nil {
			response := gin.H{"message": err.Error()}
			c.JSON(http.StatusInternalServerError, response)
			return
		}

		if !containsAnyString(string(output), filestrings[fileExt]) {
			response := gin.H{"message": "Irregular strings in file detected"}
			c.JSON(http.StatusBadRequest, response)
			return
		}

		// Image check using 'binwalk'
		// images := []string{".jpg", ".jpeg", ".png", ".gif"}
		// if tempFile.Name()

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
func extensionCheck() (map[string]string, map[string][]string, error) {
	data, err := os.ReadFile("rules.json")
	if err != nil {
		return nil, nil, fmt.Errorf("error reading file: %v", err)
	}

	var fileTypes FileTypes
	err = json.Unmarshal(data, &fileTypes)
	if err != nil {
		return nil, nil, fmt.Errorf("error parsing JSON file: %v", err)
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

	return extMap, strMap, nil
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

func hiddenFileCheck([]string, []string) bool {
	return true
}

// func hiddenFileCheck(output string) []string {
// 	lines := strings.Split(output, "\n")

// 	// Iterate through lines to process each
// 	for _, line := range lines {
// 		// Skip header and separator lines
// 		if strings.HasPrefix(line, "DECIMAL") || strings.HasPrefix(line, "------") {
// 			continue
// 		}

// 		// Extract the description part
// 		fields := strings.Fields(line)
// 		if len(fields) < 3 {
// 			continue // Skip lines that don't have enough fields
// 		}

// 		// Join all parts after the second field to form the description
// 		description := strings.Join(fields[2:], " ")

// 		// Extract the first sentence before the comma
// 		firstSentence := strings.SplitN(description, ",", 2)[0]
// 		firstSentence = strings.TrimSpace(firstSentence)
// 		lines := strings.Split(firstSentence, "\n")

// 		var fileList []string
// 		fileList = append(fileList, lines...)
// 		return fileList
// 	}
// 	return nil
// }
