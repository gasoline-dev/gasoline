package degit

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func Run(repoUrl string, branch string, extractPath string, subPath string) error {
	tarballUrl := repoUrl + "/tarball/" + branch

	data, err := downloadTarballIntoMemory(tarballUrl)
	if err != nil {
		return fmt.Errorf("unable to download tarball into memory\n%v", err)
	}

	if len(data) == 0 {
		return fmt.Errorf("downloaded data is empty")
	}

	if err := extractTarGzFromMemory(data, extractPath, subPath); err != nil {
		return fmt.Errorf("unable to extract tar gz from memory\n%v", err)
	}

	return nil
}

func downloadTarballIntoMemory(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned non-OK status: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func extractTarGzFromMemory(data []byte, dest string, subPath string) error {
	gzipStream, err := gzip.NewReader(io.NopCloser(bytes.NewReader(data)))
	if err != nil {
		return err
	}
	defer gzipStream.Close()

	tarReader := tar.NewReader(gzipStream)
	subPath = strings.TrimPrefix(subPath, "/")
	var basePath string
	foundSubPath := false

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Initialize basePath with the first directory which is usually the one with the commit hash
		if basePath == "" && header.Typeflag == tar.TypeDir {
			basePath = header.Name
		}

		// Ensure we only process files that are within the basePath and subPath
		if basePath != "" && strings.HasPrefix(header.Name, basePath) {
			relativePath := strings.TrimPrefix(header.Name, basePath)
			if !strings.HasPrefix(relativePath, subPath) {
				continue
			}

			foundSubPath = true
			targetPath := filepath.Join(dest, strings.TrimPrefix(relativePath, subPath))

			switch header.Typeflag {
			case tar.TypeDir:
				if err := os.MkdirAll(targetPath, 0755); err != nil {
					return err
				}
			case tar.TypeReg:
				outFile, err := os.Create(targetPath)
				if err != nil {
					return err
				}
				defer outFile.Close()
				if _, err := io.Copy(outFile, tarReader); err != nil {
					return err
				}
			}
		}
	}

	if !foundSubPath {
		return fmt.Errorf("subPath '%s' does not exist within the tarball", subPath)
	}

	return nil
}
