package utils

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
)

func CreateFolderIfNotExists(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0755); err != nil {
			return err
		}
	}
	return nil
}

func CreateZipFile(path string) error {
	zipFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer zipFile.Close()
	zipWriter := zip.NewWriter(zipFile)
	return zipWriter.Close()
}

// UnZipFile extracts a zip archive (zipPath) into the destination directory (destPath)
func UnZipFile(zipPath, destPath string) error {
	zipReader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zipReader.Close()

	for _, file := range zipReader.File {
		fpath := destPath + string(os.PathSeparator) + file.Name

		// Prevent ZipSlip vulnerability
		if !isPathSafe(destPath, fpath) {
			return err
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(fpath, file.Mode()); err != nil {
				return err
			}
			continue
		}

		// Make sure directories exist
		if err := os.MkdirAll(getParentDir(fpath), 0755); err != nil {
			return err
		}

		rc, err := file.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			rc.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

// isPathSafe checks for ZipSlip vulnerability by ensuring the resulting
// unpacked path is within the destination directory
func isPathSafe(dest, target string) bool {
	destAbs, _ := os.Stat(dest)
	if destAbs == nil {
		return false
	}
	absDest, _ := os.Getwd()
	if !os.IsPathSeparator(dest[len(dest)-1]) {
		absDest = dest + string(os.PathSeparator)
	} else {
		absDest = dest
	}
	return len(target) >= len(absDest) && target[:len(absDest)] == absDest
}

// getParentDir returns the parent directory of a given path.
func getParentDir(path string) string {
	i := len(path) - 1
	for i >= 0 && !os.IsPathSeparator(path[i]) {
		i--
	}
	if i > 0 {
		return path[:i]
	}
	return "."
}

// ZipFolder zips the entire srcDir into destZip (absolute or relative path)
func ZipFolder(srcDir, destZip string) error {
	zipFile, err := os.Create(destZip)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// compute header name relative to srcDir
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = rel
		if info.IsDir() {
			header.Name += "/"
		}

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := io.Copy(writer, f); err != nil {
			return err
		}
		return nil
	})
}
