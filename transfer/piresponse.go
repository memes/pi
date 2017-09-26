package transfer

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"golang.org/x/net/context"
)

const (
	CONTENT_TYPE      = "Content-Type"
	JSON_CONTENT_TYPE = "application/json"
)

type PiResponse struct {
	Index     int64  `json:"index,omitempty"`
	Digit     string `json:"digit,omitempty"`
	Cached    bool   `json:"cached,omitempty"`
	IPAddress string `json:"ipAddress,omitempty"`
}

func (p PiResponse) MarshalResponse(ctx context.Context, w http.ResponseWriter) error {
	w.Header().Set(CONTENT_TYPE, JSON_CONTENT_TYPE)
	return json.NewEncoder(w).Encode(p)
}

func (p *PiResponse) UnmarshalRequest(ctx context.Context, r *http.Response) error {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, p)
}

func MarshalError(ctx context.Context, w http.ResponseWriter, error int) {
	w.Header().Set(CONTENT_TYPE, JSON_CONTENT_TYPE)
	w.WriteHeader(error)
}
