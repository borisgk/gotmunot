package handlers

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseExifData(t *testing.T) {
	// Note: This test requires a sample JPEG file with known EXIF data
	// located at '../../testdata/exif_test.jpg' (adjusted path relative to internal/handlers).
	// The test assumes the file has the following EXIF tags:
	// - Make: "TestMake"
	// - Model: "TestModel"
	// - DateTimeOriginal: "2023:01:15 14:30:00"
	// - PixelXDimension: 800
	// - PixelYDimension: 600

	testFilePath := filepath.Join("../../testdata", "exif_test.jpg")
	file, err := os.Open(testFilePath)
	if err != nil {
		t.Fatalf("Could not open test file '%s'. Please ensure it exists and contains EXIF data. Error: %v", testFilePath, err)
	}
	defer file.Close()

	info, ok := ParseExifData(file)

	if !ok {
		t.Fatal("ParseExifData returned ok=false, expected EXIF data to be read.")
	}

	if info.Make != "NIKON CORPORATION" {
		t.Errorf("Expected Make to be 'NIKON CORPORATION', but got '%s'", info.Make)
	}

	expectedDate := time.Date(2025, 1, 13, 15, 56, 44, 0, time.UTC)
	// The parsed time won't have a location, so we compare its UTC representation.
	if !info.DateTimeOriginal.UTC().Equal(expectedDate) {
		t.Errorf("Expected DateTimeOriginal to be %v, but got %v", expectedDate, info.DateTimeOriginal)
	}

	if info.ImageWidth != 6000 {
		t.Errorf("Expected ImageWidth to be 6000, but got %d", info.ImageWidth)
	}
}
