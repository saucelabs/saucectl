package archive

import (
	"archive/zip"
	"io/ioutil"
	"os"
	"path/filepath"
)

// Zip will compress and zip up all payloads for the test project
func Zip(src, target string) error {
	output, err := os.Create(target)
	if err != nil {
		return err
	}
	defer output.Close()

	w := zip.NewWriter(output)
	defer w.Close()
	if err := addFiles(w, src, ""); err != nil {
		return err
	}

	return nil
}

func addFiles(w *zip.Writer, basePath, baseInZip string) error {
	files, err := ioutil.ReadDir(basePath)
	if err != nil {
		return err
	}
	for _, file := range files {
		if !file.IsDir() {
			data, err := ioutil.ReadFile(filepath.Join(basePath, file.Name()))
			if err != nil {
				return err
			}
			f, err := w.Create(filepath.Join(baseInZip, file.Name()))
			if err != nil {
				return err
			}
			_, err = f.Write(data)
			if err != nil {
				return err
			}
		} else {
			// recursively walk directory to add files
			if err := addFiles(w, filepath.Join(basePath, file.Name()), filepath.Join(baseInZip, file.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}
