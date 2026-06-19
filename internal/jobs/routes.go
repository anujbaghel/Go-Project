package jobs

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"
)

func Routes(r chi.Router, client *asynq.Client) {
	r.Post("/jobs/credit", func(rw http.ResponseWriter, req *http.Request) {
		var p CreditPayload
		if err := json.NewDecoder(req.Body).Decode(&p); err != nil || p.ConstraintID == "" {
			http.Error(rw, "uid, amount, constraintId required", http.StatusBadRequest)
			return
		}

		id, err := EnqueueCredit(client, p)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(map[string]string{"jobId": id})
	})
}
