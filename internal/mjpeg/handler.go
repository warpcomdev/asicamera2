package mjpeg

import (
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"time"

	"github.com/warpcomdev/asicamera2/internal/jpeg"
)

type Session interface {
	Next(frameNumber uint64) (*jpeg.JpegFrame, uint64, jpeg.FrameStatus)
}

type SessionManager interface {
	Acquire() Session
	Release(Session)
}

// Track https://github.com/golang/go/issues/54136 for improvements on timeout handling
func Handler(mgr SessionManager) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		protocol := r.Proto
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "Protocol Not Supported", http.StatusMethodNotAllowed)
			return
		}

		conn, rw, err := hijacker.Hijack()
		if err != nil {
			http.Error(w, "Hijacking failed", http.StatusMethodNotAllowed)
			return
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(5 * time.Second)) // 5 seconds deadline to set the streaming up

		session := mgr.Acquire()
		defer mgr.Release(session)

		// keep monitoring the conn while increasing the read deadline
		keepAlive := make(chan struct{})
		go func() {
			defer close(keepAlive)
			one := make([]byte, 1)
			for {
				if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
					return
				}
				if _, err := rw.Read(one); errors.Is(err, io.EOF) {
					return
				}
				rw.Discard(rw.Available())
			}
		}()

		mimeWriter := multipart.NewWriter(rw)
		defer mimeWriter.Close()

		// Write the HTTP headers by hand
		rw.WriteString(protocol)
		rw.WriteString(" 200 OK\n")
		rw.WriteString("Connection: close\n")
		rw.WriteString("Cache-Control: no-store, no-cache\n")
		rw.WriteString("Content-Type: ")
		rw.WriteString(fmt.Sprintf("multipart/x-mixed-replace;boundary=%s", mimeWriter.Boundary()))
		rw.WriteString("\n\n")
		rw.Flush()

		// Build a channel for pushing frames
		frames := make(chan *jpeg.JpegFrame)
		go func() {
			defer close(frames)
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
				case <-keepAlive:
					log.Print("keepAlive expired")
					frame.Done()
					return
				case frames <- frame:
					currentFrame = frameNum + 1
				}
			}
		}()

		for frame := range frames {
			err = func() error { // embedded in a function, to be able to defer
				defer frame.Done()
				conn.SetWriteDeadline(time.Now().Add(2 * time.Second)) // max 2 seconds to deliver the payload
				partHeader := make(textproto.MIMEHeader)
				partHeader.Add("Content-Type", "image/jpeg")

				partWriter, err := mimeWriter.CreatePart(partHeader)
				if err != nil {
					return fmt.Errorf("mjpeg: createPart: %w", err)
				}

				if _, err := partWriter.Write(frame.Slice()); err != nil {
					return fmt.Errorf("mjpeg: write: %w", err)
				}

				if err := rw.Flush(); err != nil {
					return fmt.Errorf("mjpeg: flush: %w", err)
				}
				return nil
			}()
			if err != nil {
				// If we missed a frame, we don't know how is the stream
				// actually ... better disconnect and force start again
				log.Print(err)
				// exhaust the frame goroutine
				conn.Close()
				for _ = range frames {
					frame.Done()
				}
				return
			}
		}
	})
}
