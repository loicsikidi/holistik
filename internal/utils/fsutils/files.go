package fsutils

import (
	"io"
	"os"

	"github.com/loicsikidi/holistik/internal/utils"
	"github.com/loicsikidi/sentinel"
)

const (
	// DefaultMaxFileSize is the default maximum file size for [ReadFileSecure] (5 MiB).
	DefaultMaxFileSize int64 = 5 * 1024 * 1024
	// DefaultMaxHTTPGetSize is the default maximum file size for HTTP GET requests (5 MiB).
	DefaultMaxHTTPGetSize int64 = DefaultMaxFileSize
)

// FileExists checks if a file exists and is not a directory.
func FileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// DirExists checks if a directory exists.
func DirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// ReadFile reads the content of a file with a maximum size limit.
//
// If filename is "-", reads from stdin.
// Default maximum size is [DefaultMaxFileSize], but can be overridden by providing a custom maxSize in bytes.
func ReadFile(filename string, optionalMaxSize ...int64) ([]byte, error) {
	maxSize := utils.OptionalArgWithDefault(optionalMaxSize, DefaultMaxFileSize)
	var reader io.Reader
	if filename == "-" {
		reader = os.Stdin
	} else {
		file, err := os.Open(filename)
		if err != nil {
			return nil, sentinel.ConvertSystemError(err)
		}
		defer file.Close() //nolint:errcheck
		reader = file
	}

	limitedReader := io.LimitReader(reader, maxSize+1)

	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, err
	}

	if int64(len(data)) > maxSize {
		return nil, sentinel.BadParameter("file too large: exceeds %d bytes", maxSize)
	}

	return data, nil
}
