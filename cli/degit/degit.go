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
)

func run(repoUrl string, branch string, extractPath string) error {
	tarballUrl := repoUrl + "/tarball/" + branch

	data, err := downloadTarballIntoMemory(tarballUrl)
	if err != nil {
		return fmt.Errorf("unable to download tarball into memory\n%v", err)
	}

	if err := extractTarGzFromMemory(data, extractPath); err != nil {
		return fmt.Errorf("unable to  extract tar gz from memory\n%v", err)
	}

	return nil
}

func downloadTarballIntoMemory(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func extractTarGzFromMemory(data []byte, dest string) error {
	gzipStream, err := gzip.NewReader(io.NopCloser(bytes.NewReader(data)))
	if err != nil {
		return err
	}
	defer gzipStream.Close()

	tarReader := tar.NewReader(gzipStream)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(filepath.Join(dest, header.Name), 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			outFile, err := os.Create(filepath.Join(dest, header.Name))
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		}
	}

	return nil
}
