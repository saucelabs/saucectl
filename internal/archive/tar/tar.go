package tar

import (
	"archive/tar"
	"bytes"
	"github.com/rs/zerolog/log"
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

// addFileToArchive adds a file into an archive.
func addFileToArchive(fileName string, fileInfo os.FileInfo, rootFolder string, matcher sauceignore.Matcher, opts Options, w *tar.Writer) error {
	if matcher.Match(strings.Split(fileName, string(os.PathSeparator)), fileInfo.IsDir()) {
		log.Debug().Str("fileName", fileName).Msg("Ignoring file")
		return nil
	}
	header, err := tar.FileInfoHeader(fileInfo, fileName)
	if err != nil {
		return err
	}

	if opts.Permission != nil {
		header.Mode = opts.Permission.Mode
		header.Uid = opts.Permission.UID
		header.Gid = opts.Permission.GID
	}

	relName := filepath.Base(fileName)
	if rootFolder != "" {
		relName, err = filepath.Rel(rootFolder, fileName)
		if err != nil {
			return err
		}
	}

	relName = filepath.ToSlash(relName)
	header.Name = relName

	if fileInfo.Mode().Type() == os.ModeSymlink {
		linkTarget, err := filepath.EvalSymlinks(fileName)
		if err != nil {
			return err
		}
		relLinkName, err := filepath.Rel(filepath.Dir(fileName), linkTarget)
		if err != nil {
			return err
		}
		header.Linkname = relLinkName
	}

	if err := w.WriteHeader(header); err != nil {
		return err
	}

	if fileInfo.IsDir() || fileInfo.Mode().Type() == os.ModeSymlink {
		return nil
	}

	srcFile, err := os.Open(fileName)
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

// Archive archives the resource and exclude files and folders based on sauceignore logic.
func Archive(src string, matcher sauceignore.Matcher, opts Options) (io.Reader, error) {
	bb := new(bytes.Buffer)
	w := tar.NewWriter(bb)
	defer w.Close()

	infoSrc, err := os.Stat(src)
	if err != nil {
		return nil, err
	}

	// Single file addition
	if !infoSrc.IsDir() {
		err = addFileToArchive(src, infoSrc, "", matcher, opts, w)
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(bb.Bytes()), nil
	}

	walker := func(file string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		err = addFileToArchive(file, fileInfo, src, matcher, opts, w)
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
