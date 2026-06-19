package worker

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func Routes(r chi.Router, w *Worker) {
	r.Post("/tournaments/{ltid}/payout", func(rw http.ResponseWriter, req *http.Request) {
		ltid, _ := strconv.ParseInt(chi.URLParam(req, "ltid"), 10, 64)
		if err := w.processTournament(req.Context(), ltid); err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(map[string]string{"status": "payout swept"})
	})
}
