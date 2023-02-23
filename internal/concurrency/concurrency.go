package concurrency

// SplitTestFiles splits test files into groups to match concurrency
func SplitTestFiles(files []string, concurrency int) [][]string {
	if concurrency == 1 {
		return [][]string{files}
	}
	if concurrency > len(files) {
		concurrency = len(files)
	}
	buckets := make([][]string, concurrency)
	for i := 0; i < len(files); i++ {
		buckets[i%concurrency] = append(buckets[i%concurrency], files[i])
	}

	return buckets
}
