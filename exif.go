package main

import (
	"io"
	"log"
	"time"
	"fmt"

	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/mknote"
)
// ExifInfo holds a comprehensive set of parsed EXIF data.
// Fields are basic types (string, float64, etc.) and will be zero-valued if not present.
type ExifInfo struct {
	// Image Description
	ImageDescription string
	ImageWidth       uint32
	ImageLength      uint32
	XResolution      float64
	YResolution      float64
	ResolutionUnit   uint16
	Make             string
	Model            string
	Orientation      uint16
	Software         string
	DateTime         time.Time
	Artist           string
	Copyright        string

	// Exposure Details
	ExposureTime      string // e.g., "1/125"
	ExposureProgram   uint16
	FNumber           float64
	ISOSpeedRatings   uint16
	ShutterSpeedValue string
	ApertureValue     float64
	ExposureBiasValue string
	MaxApertureValue  float64
	MeteringMode      uint16
	LightSource       uint16
	Flash             uint16

	// Lens Details
	FocalLength           float64
	FocalLengthIn35mmFilm uint16
	LensMake              string
	LensModel             string

	// Timestamps
	DateTimeOriginal  time.Time
	DateTimeDigitized time.Time
	SubSecTime        string

	// GPS
	GPSLatitudeRef  string
	GPSLatitude     float64
	GPSLongitudeRef string
	GPSLongitude    float64
	GPSAltitudeRef  uint8
	GPSAltitude     float64
	GPSTimeStamp    time.Time
	GPSSpeedRef     string
	GPSSpeed        float64
	GPSImgDirection float64
}

// Helper to get a string value from an EXIF tag.
func getString(x *exif.Exif, name exif.FieldName) string {
	tag, err := x.Get(name)
	if err != nil {
		return ""
	}
	val, err := tag.StringVal()
	if err != nil {
		return ""
	}
	return val
}

// Helper to get a uint16 value from an EXIF tag.
func getUint16(x *exif.Exif, name exif.FieldName) uint16 {
	tag, err := x.Get(name)
	if err != nil {
		return 0
	}
	// The library often returns int64, so we get that and cast.
	val, err := tag.Int64(0)
	if err != nil {
		return 0
	}
	return uint16(val)
}

// Helper to get a uint32 value from an EXIF tag.
func getUint32(x *exif.Exif, name exif.FieldName) uint32 {
	tag, err := x.Get(name)
	if err != nil {
		return 0
	}
	// The library often returns int64, so we get that and cast.
	val, err := tag.Int64(0)
	if err != nil {
		return 0
	}
	return uint32(val)
}

// Helper to get a float64 value from a rational EXIF tag.
func getFloat(x *exif.Exif, name exif.FieldName) float64 {
	tag, err := x.Get(name)
	if err != nil {
		return 0
	}
	numer, denom, err := tag.Rat2(0)
	if err != nil || denom == 0 {
		return 0
	}
	return float64(numer) / float64(denom)
}

// Helper to get a string representation of a rational EXIF tag.
func getRatString(x *exif.Exif, name exif.FieldName) string {
	tag, err := x.Get(name)
	if err != nil {
		return ""
	}
	return tag.String()
}

// Helper to get a time.Time value from an EXIF tag.
func getDateTime(x *exif.Exif, name exif.FieldName) time.Time {
	tag, err := x.Get(name)
	if err != nil {
		return time.Time{}
	}
	// EXIF time format is "YYYY:MM:DD HH:MM:SS"
	const layout = "2006:01:02 15:04:05"
	valStr, err := tag.StringVal()
	if err != nil {
		return time.Time{}
	}
	t, err := time.Parse(layout, valStr)
	if err != nil {
		return time.Time{}
	}
	return t
}

// getGpsDateTime parses the GPSDateStamp and GPSTimeStamp tags to return a time.Time.
// GPS time is stored in UTC.
func getGpsDateTime(x *exif.Exif) time.Time {
	dateTag, err := x.Get(exif.GPSDateStamp)
	if err != nil {
		return time.Time{}
	}
	dateStr, err := dateTag.StringVal() // Format: "YYYY:MM:DD"
	if err != nil {
		return time.Time{}
	}

	timeTag, err := x.Get(exif.GPSTimeStamp)
	if err != nil {
		return time.Time{}
	}

	// GPSTimeStamp contains 3 rationals for h, m, s.
	h, _, errH := timeTag.Rat2(0)
	m, _, errM := timeTag.Rat2(1)
	s, _, errS := timeTag.Rat2(2)
	if errH != nil || errM != nil || errS != nil {
		return time.Time{}
	}

	fullDateTimeStr := fmt.Sprintf("%s %02d:%02d:%02d", dateStr, h, m, s)
	t, err := time.Parse("2006:01:02 15:04:05", fullDateTimeStr)
	if err != nil {
		return time.Time{}
	}
	return t
}

// parseExifData reads an io.Reader and extracts a comprehensive set of EXIF metadata.
// It returns an ExifInfo struct and a boolean indicating if any EXIF data was read.
func parseExifData(r io.Reader) (ExifInfo, bool) {
	var info ExifInfo

	exif.RegisterParsers(mknote.All...)
	exifData, err := exif.Decode(r)
	if err != nil {
		log.Printf("Could not decode EXIF data: %v", err)
		return info, false
	}

	// Image Description
	info.ImageDescription = getString(exifData, exif.ImageDescription)
	info.ImageWidth = getUint32(exifData, exif.PixelXDimension)
	info.ImageLength = getUint32(exifData, exif.PixelYDimension)
	info.XResolution = getFloat(exifData, exif.XResolution)
	info.YResolution = getFloat(exifData, exif.YResolution)
	info.ResolutionUnit = getUint16(exifData, exif.ResolutionUnit)
	info.Make = getString(exifData, exif.Make)
	info.Model = getString(exifData, exif.Model)
	info.Orientation = getUint16(exifData, exif.Orientation)
	info.Software = getString(exifData, exif.Software)
	info.DateTime, _ = exifData.DateTime()
	info.Artist = getString(exifData, exif.Artist)
	info.Copyright = getString(exifData, exif.Copyright)

	// Exposure Details
	info.ExposureTime = getRatString(exifData, exif.ExposureTime)
	info.ExposureProgram = getUint16(exifData, exif.ExposureProgram)
	info.FNumber = getFloat(exifData, exif.FNumber)
	info.ISOSpeedRatings = getUint16(exifData, exif.ISOSpeedRatings)
	info.ShutterSpeedValue = getRatString(exifData, exif.ShutterSpeedValue)
	info.ApertureValue = getFloat(exifData, exif.ApertureValue)
	info.ExposureBiasValue = getRatString(exifData, exif.ExposureBiasValue)
	info.MaxApertureValue = getFloat(exifData, exif.MaxApertureValue)
	info.MeteringMode = getUint16(exifData, exif.MeteringMode)
	info.LightSource = getUint16(exifData, exif.LightSource)
	info.Flash = getUint16(exifData, exif.Flash)

	// Lens Details
	info.FocalLength = getFloat(exifData, exif.FocalLength)
	info.FocalLengthIn35mmFilm = getUint16(exifData, exif.FocalLengthIn35mmFilm)
	info.LensMake = getString(exifData, exif.LensMake)
	info.LensModel = getString(exifData, exif.LensModel)

	// Timestamps
	info.DateTimeOriginal = getDateTime(exifData, exif.DateTimeOriginal)
	info.DateTimeDigitized = getDateTime(exifData, exif.DateTimeDigitized)
	info.SubSecTime = getString(exifData, exif.SubSecTime)

	// GPS
	info.GPSLatitude, info.GPSLongitude, _ = exifData.LatLong()
	info.GPSAltitude = getFloat(exifData, exif.GPSAltitude)
	info.GPSTimeStamp = getGpsDateTime(exifData)
	info.GPSSpeedRef = getString(exifData, exif.GPSSpeedRef)
	info.GPSSpeed = getFloat(exifData, exif.GPSSpeed)
	info.GPSImgDirection = getFloat(exifData, exif.GPSImgDirection)

	return info, true
}