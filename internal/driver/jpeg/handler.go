package jpeg

import (
	"net/http"

	"go.uber.org/zap"
)

// Track https://github.com/golang/go/issues/54136 for improvements on timeout handling
func Handler(logger *zap.Logger, mgr *SessionManager) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		session, err := mgr.Acquire(logger)
		if err != nil {
			logger.Error("Acquiring session failed", zap.Error(err))
			http.Error(w, "Acquiring session failed", http.StatusInternalServerError)
			return
		}
		defer mgr.Done()

		frame, _, status := session.Next(1)
		if frame == nil { // session died
			logger.Info("session terminated, disconnecting client")
			return
		}
		defer frame.Done()
		if status != FrameReady {
			http.Error(w, "Capturing frame failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		w.Write(frame.Slice())
	})
}
