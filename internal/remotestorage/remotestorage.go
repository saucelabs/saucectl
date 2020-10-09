package remotestorage

// RemoteStorage defines functions to interact with application storage service
type RemoteStorage interface {
	Upload(fileName, formType string) (StorageResponse, error)
}

// StorageResponse defines response from remote storage service
type StorageResponse struct {
	Item struct {
		ID    string `json:"id"`
		Owner struct {
			ID    string `json:"id"`
			OrgID string `json:"org_id"`
		} `json:"owner"`
		Name            string      `json:"name"`
		UploadTimestamp int         `json:"upload_timestamp"`
		Etag            string      `json:"etag"`
		Kind            string      `json:"kind"`
		GroupID         int         `json:"group_id"`
		Metadata        interface{} `json:"metadata"`
		Access          struct {
			TeamIds []string `json:"team_ids"`
			OrgIds  []string `json:"org_ids"`
		} `json:"access"`
	} `json:"item"`
}
