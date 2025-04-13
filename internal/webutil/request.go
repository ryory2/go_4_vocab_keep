package webutil

import (
	"encoding/json"
	"log"
	"net/http"

	"go_4_vocab_keep/internal/model" // modelパッケージのインポートパスに修正 (go.modに合わせてください)
)

// DecodeJSONBody はリクエストボディをデコードします
func DecodeJSONBody(r *http.Request, dst interface{}) error {
	if r.Body == nil {
		return model.ErrInvalidInput
	}
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	err := decoder.Decode(dst)
	if err != nil {
		log.Printf("Error decoding JSON body: %v", err)
		return model.ErrInvalidInput
	}
	// TODO: バリデーション実行
	return nil
}
