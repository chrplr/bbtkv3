package main

import (
	"fmt"
	"os"
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

// GetNextBase returns the next available base name of the form "{name}-NNN",
// checking that none of the three output files (.dat, -dscevents.csv, -events.csv) already exist.
func GetNextBase(name string) string {
	for i := 1; ; i++ {
		base := fmt.Sprintf("%s-%03d", name, i)
		if !fileExists(base+".dat") && !fileExists(base+"-dscevents.csv") && !fileExists(base+"-events.csv") {
			return base
		}
	}
}
