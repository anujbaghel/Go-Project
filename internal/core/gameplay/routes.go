package gameplay

import (
	"Go-project/internal/core/gateway"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/go-chi/chi/v5"
)

func Routes(r chi.Router, svc *Service, secret []byte) {
	r.Get("/play", func(rw http.ResponseWriter, req *http.Request) {
		uid, err := gateway.ParseToken(secret, req.URL.Query().Get("token"))
		if err != nil {
			http.Error(rw, "Unathorized", http.StatusUnauthorized)
			return
		}
		ltid, _ := strconv.ParseInt(req.URL.Query().Get("ltid"), 10, 64)

		// upgrade HTTP → WebSocket. InsecureSkipVerify disables the Origin check
		// (fine for the lab; in prod you'd set OriginPatterns instead).
		c, err := websocket.Accept(rw, req, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer c.CloseNow()

		ctx := req.Context()
		for { // Read messages until the client disconnects
			var msg struct {
				Score int `json:"score"`
			}
			if err := wsjson.Read(ctx, c, &msg); err != nil {
				slog.Error("Error reading", "ltid", ltid, "uid", uid, "err", err)
				return // client closed or error → end the loop, defer closes the socket
			}
			if err := svc.RecordScore(ctx, ltid, uid, msg.Score); err != nil {
				slog.Error("record score", "ltid", ltid, "uid", uid, "err", err)
				_ = wsjson.Write(ctx, c, map[string]string{"error": "Could not write"})
				continue
			}
			top, _ := svc.TopN(ctx, ltid, 3)
			_ = wsjson.Write(ctx, c, map[string]any{"ok": true, "top": top})
		}
	})

	r.Get("/tournaments/{ltid}/leaderboard", func(rw http.ResponseWriter, req *http.Request) {
		ltid, _ := strconv.ParseInt(chi.URLParam(req, "ltid"), 10, 64)
		top, err := svc.TopN(req.Context(), ltid, 10)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(map[string]any{"result": top})
	})
}
