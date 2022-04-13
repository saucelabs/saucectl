package testfiles

// ExcludeFiles returns file list which excluding given excluded file list
func ExcludeFiles(testFiles, excludedList []string) []string {
	var files []string
	for _, t := range testFiles {
		excluded := false
		for _, e := range excludedList {
			if t == e {
				excluded = true
				break
			}
		}
		if !excluded {
			files = append(files, t)
		}
	}

	return files
}
