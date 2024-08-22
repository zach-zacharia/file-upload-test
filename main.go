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
	Extension   string   `json:"extension"`
	Description string   `json:"description"`
	Strings     []string `json:"strings"`
}

// AllowedFiles represents the collection of file types
type Rules struct {
	AllowedFiles      []FileType `json:"allowed_files"`
	ForbiddenKeywords []string   `json:"forbidden_keywords"`
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
		fileinfo, filestrings, forbiddenKeywords, err := extensionCheck()
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
		} else if ok {
			color.Green("File is allowed. Proceeding to duplicate file for further scanning...\n\n")
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
		} else if containsContent(string(output), contentDescription) {
			color.Green("File description matches the file extension. Proceeding...\n\n")
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
			response := gin.H{"message": "Mismatching or irregular strings in file detected"}
			c.JSON(http.StatusForbidden, response)
			return
		} else if containsAnyString(string(output), filestrings[fileExt]) {
			color.Green("No unusual strings found. Proceeding...\n\n")
		}

		// Check for forbidden files or keywords
		code, found := checkForbidden(tempFile.Name(), forbiddenKeywords)
		if found {
			color.Red(code)
			response := gin.H{"message": "Forbidden files and/or tags are found"}
			c.JSON(http.StatusForbidden, response)
			return
		} else if !found {
			color.Green(code)
		}

		// clamAV check
		if clamavCheck(tempFile.Name()) {
			response := gin.H{"message": "This file contains a virus. Aborting upload."}
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
	for _, ft := range rules.AllowedFiles {
		contentMap[ft.Extension] = ft.Description
	}

	stringMap := make(map[string][]string)
	for _, ft := range rules.AllowedFiles {
		stringMap[ft.Extension] = ft.Strings
	}

	forbiddenFiles := rules.ForbiddenKeywords

	return contentMap, stringMap, forbiddenFiles, nil
}

func containsContent(output, contentDescription string) bool {

	// fmt.Printf("Command output: %s\n", output)                           // Debug print
	// fmt.Printf("Expected content description: %s\n", contentDescription) // Debug print

	normalizedContent := normalizeContentDescription(contentDescription)
	return strings.Contains(strings.ToLower(output), normalizedContent)
}

func containsAnyString(output string, expectedStrings []string) bool {
	// fmt.Printf("Command output: %s\n", output)            // Debug print
	// fmt.Printf("Expected strings: %v\n", expectedStrings) // Debug print

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

func clamavCheck(file string) bool {
	color.Yellow("Beginning clamAV scan...\n")
	cmd := exec.Command("clamscan", file)
	output, _ := cmd.CombinedOutput()
	result := string(output)
	if strings.Contains(result, "FOUND") {
		color.Red("A virus has been spotted inside the file! Aborting upload...\n")
		return true
	} else {
		color.Green("No virus has been spotted. Proceeding...\n")
		return false
	}
}

func checkForbidden(file string, forbiddenList []string) (results string, foundForbidden bool) {
	var codes = []string{"Errored", "No forbidden found", "Found forbidden"}
	color.Yellow("Beginning exiftool scan...\n")
	cmd := exec.Command("exiftool", file)
	output, err := cmd.Output()
	switch {
	case err != nil:
		result := fmt.Sprint(codes[0], " during exiftool scan\n")
		return result, true
	case containsAnyString(string(output), forbiddenList):
		result := fmt.Sprint(codes[2], " during exiftool scan\n")
		return result, true
	}

	color.Green("Nothing unusual found during exiftool check. Proceeding...\n\n")
	color.Yellow("Beginning binwalk scan...\n")
	cmd = exec.Command("binwalk", file)
	output, err = cmd.Output()
	switch {
	case err != nil:
		result := fmt.Sprint(codes[0], " during binwalk scan\n")
		return result, true
	case containsAnyString(string(output), forbiddenList):
		result := fmt.Sprint(codes[2], " during binwalk scan\n")
		return result, true
	}

	color.Green("Nothing unusual found during binwalk check. Proceeding...\n\n")

	result := fmt.Sprint(codes[1], " during scans. Proceeding...\n\n")
	return result, false
}
