package mjpeg

import (
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"

	"github.com/warpcomdev/asicamera2/internal/jpeg"
)

type Session interface {
	Next(frameNumber uint64) (*jpeg.JpegFrame, uint64, jpeg.FrameStatus)
}

type SessionManager interface {
	Acquire() Session
	Release(Session)
}

func Handler(mgr SessionManager) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		session := mgr.Acquire()
		defer mgr.Release(session)

		mimeWriter := multipart.NewWriter(w)
		defer mimeWriter.Close()

		w.Header().Add("Connection", "close")
		w.Header().Add("Cache-Control", "no-store, no-cache")
		w.Header().Add("Content-Type", fmt.Sprintf("multipart/x-mixed-replace;boundary=%s", mimeWriter.Boundary()))

		frames := make(chan *jpeg.JpegFrame)
		go func() {
			defer close(frames)
			cn := w.(http.CloseNotifier).CloseNotify()
			var currentFrame uint64 = 1
			for {
				frame, frameNum, status := session.Next(currentFrame)
				if frame == nil { // session died
					log.Print("session terminated, disconnecting client")
					return
				}
				if status != jpeg.FrameReady {
					log.Printf("frame %d is not ready (status %d), skipping", frameNum, status)
					frame.Done()
					continue
				}
				select {
				case <-cn:
					log.Print("multipart client disconnected")
					frame.Done()
					return
				case frames <- frame:
					currentFrame = frameNum + 1
				}
			}
		}()

		for frame := range frames {
			func() { // embedded in a function, to be able to defer
				defer frame.Done()
				partHeader := make(textproto.MIMEHeader)
				partHeader.Add("Content-Type", "image/jpeg")

				partWriter, err := mimeWriter.CreatePart(partHeader)
				if err != nil {
					log.Printf("mjpeg: createPart: %v", err)
					return
				}

				if _, err := partWriter.Write(frame.Slice()); err != nil {
					log.Printf("mjpeg: write: %v", err)
					return
				}
			}()
		}
	})
}
