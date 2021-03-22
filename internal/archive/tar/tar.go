package tar

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/saucelabs/saucectl/internal/sauceignore"
)

// Options represents the options applied when archiving files.
type Options struct {
	Permission *Permission
}

// Permission represents the permissions applied when archiving files.
type Permission struct {
	Mode int64 // Permission and mode bits
	UID  int   // User ID of owner
	GID  int   // Group ID of owner
}

// Archive archives the resource and exclude files and folders based on sauceignore logic.
func Archive(src string, matcher sauceignore.Matcher, opts Options) (io.Reader, error) {
	bb := new(bytes.Buffer)
	w := tar.NewWriter(bb)
	defer w.Close()

	infoSrc, err := os.Stat(src)
	if err != nil {
		return nil, err
	}

	baseDir := ""
	if infoSrc.IsDir() {
		baseDir = src
	}

	walker := func(file string, fileInfo os.FileInfo, err error) error {
		// Only will be applied if we have .sauceignore file and have patterns to exclude files and folders
		if matcher.Match(strings.Split(file, string(os.PathSeparator)), fileInfo.IsDir()) {
			return nil
		}

		header, err := tar.FileInfoHeader(fileInfo, file)
		if err != nil {
			return err
		}

		if opts.Permission != nil {
			header.Mode = opts.Permission.Mode
			header.Uid = opts.Permission.UID
			header.Gid = opts.Permission.GID
		}

		if baseDir != "" {
			relName, err := filepath.Rel(baseDir, file)
			if err != nil {
				return err
			}
			header.Name = filepath.Join(baseDir, relName)
		}

		if err := w.WriteHeader(header); err != nil {
			return err
		}

		if fileInfo.IsDir() {
			return nil
		}

		srcFile, err := os.Open(file)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		_, err = io.Copy(w, srcFile)
		if err != nil {
			return err
		}

		return nil
	}

	if err := filepath.Walk(src, walker); err != nil {
		return nil, err
	}

	return bytes.NewReader(bb.Bytes()), nil
}
