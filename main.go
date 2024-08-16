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

	"github.com/fatih/color"
	"github.com/gin-gonic/gin"
)

// FileType represents each file type with an extension and content description
type FileType struct {
	Extension string   `json:"extension"`
	Content   string   `json:"content"`
	Strings   []string `json:"strings"`
}

// FileTypes represents the collection of file types
type Rules struct {
	FileTypes      []FileType `json:"file_types"`
	ForbiddenFiles []string   `json:"forbidden_files"`
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
			response := gin.H{"message": err.Error()}
			c.JSON(http.StatusBadRequest, response)
			return
		}

		fileExt := filepath.Ext(fileHeader.Filename)
		fileinfo, filestrings, forbiddenFiles, err := extensionCheck()
		if err != nil {
			response := gin.H{"message": err.Error()}
			c.JSON(http.StatusInternalServerError, response)
			return
		}

		// Extension check
		contentDescription, ok := fileinfo[fileExt]
		if !ok {
			color.Red("\nForbidden file extension found!")
			response := gin.H{"message": "File extension is not allowed"}
			c.JSON(http.StatusForbidden, response)
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
			color.Red("\nFile description inconsistency found!")
			response := gin.H{"message": "File description does not match the expected type"}
			c.JSON(http.StatusForbidden, response)
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
			color.Red("\nIrregular strings found inside file!")
			response := gin.H{"message": "Irregular strings in file detected"}
			c.JSON(http.StatusForbidden, response)
			return
		}

		// Checking hidden files using binwalk
		cmd = exec.Command("binwalk", tempFile.Name())
		output, err = cmd.Output()
		if err != nil {
			response := gin.H{"message": err.Error()}
			c.JSON(http.StatusInternalServerError, response)
			return
		}

		file_list := hiddenFileCheck(string(output))
		fmt.Print(file_list)

		if containsAnyString(file_list, forbiddenFiles) {
			color.Red("Forbidden hidden files found!")
			response := gin.H{"message": "Forbidden hidden files detected"}
			c.JSON(http.StatusForbidden, response)
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

		color.Green("\nNo anomaly found, uploading file...")
		response := gin.H{"message": "File uploaded successfully"}

		c.JSON(http.StatusOK, response)
	})

	server.Run(":2000")
}

// extensionCheck reads the JSON file and returns a map of file extensions to content descriptions
func extensionCheck() (map[string]string, map[string][]string, []string, error) {
	data, err := os.ReadFile("rules.json")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error reading file: %v", err)
	}

	var rules Rules
	err = json.Unmarshal(data, &rules)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error parsing JSON file: %v", err)
	}

	contentMap := make(map[string]string)
	for _, ft := range rules.FileTypes {
		contentMap[ft.Extension] = ft.Content
	}

	stringMap := make(map[string][]string)
	for _, ft := range rules.FileTypes {
		stringMap[ft.Extension] = ft.Strings
	}

	forbiddenFiles := rules.ForbiddenFiles

	return contentMap, stringMap, forbiddenFiles, nil
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

func hiddenFileCheck(output string) string {
	lines := strings.Split(output, "\n")

	// Iterate through lines to process each
	for _, line := range lines {
		// Skip header and separator lines
		if strings.HasPrefix(line, "DECIMAL") || strings.HasPrefix(line, "------") {
			continue
		}

		// Extract the description part
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue // Skip lines that don't have enough fields
		}

		// Join all parts after the second field to form the description
		description := strings.Join(fields[2:], " ")

		// Extract the first sentence before the comma
		firstSentence := strings.SplitN(description, ",", 2)[0]
		firstSentence = strings.TrimSpace(firstSentence)

		return firstSentence
	}
	return ""
}
