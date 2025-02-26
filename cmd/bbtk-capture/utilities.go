package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// fileExists checks if a file with the given filename exists.
// It returns true if the file exists, and false otherwise.
// If an error occurs while trying to retrieve the file information,
// it assumes the file does not exist.
func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// changeExtension changes the extension of the given filename to the new extension provided.
// If the new extension does not start with a ".", it will be added automatically.
//
// Parameters:
//   - filename: The original filename whose extension needs to be changed.
//   - newExt: The new extension to be applied to the filename.
//
// Returns:
//
//	A string representing the filename with the new extension.
func changeExtension(filename string, newExt string) string {
	// Get the file extension
	ext := filepath.Ext(filename)

	// Remove the old extension and add the new one
	// If newExt doesn't start with ".", add it
	if !strings.HasPrefix(newExt, ".") {
		newExt = "." + newExt
	}

	return strings.TrimSuffix(filename, ext) + newExt
}

func GetNextFileName(basename string) string {
	ext := filepath.Ext(basename)
	name := strings.TrimSuffix(basename, ext)
	filename := fmt.Sprintf("%s-%03d%s", name, 1, ext)

	for i := 2; fileExists(filename); i++ {
		filename = fmt.Sprintf("%s-%03d%s", name, i, ext)
	}
	return filename
}

func WriteText(basename string, text string) (string, error) {
	var filename string = GetNextFileName(basename)

	f, err := os.Create(filename)
	if err != nil {
		return filename, err
	}

	defer f.Close()

	_, err = f.WriteString(text)

	return filename, err
}
