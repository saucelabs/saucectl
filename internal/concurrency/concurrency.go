package concurrency

// BinPack splits items into groups to match concurrency
func BinPack(items []string, concurrency int) [][]string {
	if concurrency == 1 {
		return [][]string{items}
	}
	if concurrency > len(items) {
		concurrency = len(items)
	}
	buckets := make([][]string, concurrency)
	for i := 0; i < len(items); i++ {
		buckets[i%concurrency] = append(buckets[i%concurrency], items[i])
	}

	return buckets
}
