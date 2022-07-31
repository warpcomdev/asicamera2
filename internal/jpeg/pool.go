package jpeg

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

type FrameStatus int

const (
	FrameEmpty       FrameStatus = iota // Frame has no valid data
	FrameCompressing                    // Frame is being compressed
	FrameReady                          // Frame is ready to stream
	FrameFailed                         // Frame failed to ocmpress
	FrameStuck                          // frame was not processed because some reader had it locked
)

// Source of frames
type Source interface {
	Next(ctx context.Context, img *Image) // get next frame
}

// Compressed frame
type Frame struct {
	number   uint64
	status   FrameStatus
	features JpegFeatures
	image    Image
	group    sync.WaitGroup // readers sending the image to clients
}

// drainer of Sources
type drainer struct {
	currentFrame uint64
	rawFrames    chan *Image
	NumRawFrames int
}

// Setup the drainer with initial buffers
func (d *drainer) Setup(numRawFrames int) {
	d.NumRawFrames = numRawFrames
	d.currentFrame = 0
	d.rawFrames = make(chan *Image, numRawFrames)
	for i := 0; i < numRawFrames; i++ {
		d.rawFrames <- &Image{}
	}
}

// Release an image back to the free image list
func (d *drainer) Release(img *Image) {
	d.rawFrames <- img
}

// compression task
type task struct {
	frameNumber uint64
	rawFrame    *Image // input buffer
}

// Reset the drainer count to frame number 0
func (d *drainer) Reset(frameNumber uint64) {
	atomic.StoreUint64(&(d.currentFrame), frameNumber)
}

// Drain a source by sending a task per frame, until context is cancelled
func (d *drainer) Drain(ctx context.Context, source Source, tasks chan task) {
	var frameNumber uint64 = atomic.LoadUint64(&(d.currentFrame))
	for {
		var rawFrame *Image
		select {
		case <-ctx.Done():
			d.rawFrames <- rawFrame
			return
		case rawFrame = <-d.rawFrames:
		}
		source.Next(ctx, rawFrame)
		atomic.StoreUint64(&(d.currentFrame), frameNumber)
		select {
		case <-ctx.Done():
			d.rawFrames <- rawFrame
			return
		case tasks <- task{frameNumber: frameNumber, rawFrame: rawFrame}:
			frameNumber += 1
		}
	}
}

// CurrentFrame returns the current frame being processed
func (d *drainer) CurrentFrame() uint64 {
	return atomic.LoadUint64(&(d.currentFrame))
}

// Pool of compression gophers
type Pool struct {
	// Compression parameters
	subsampling Subsampling
	quality     int
	flags       int

	// Compression gopher group
	drainer drainer        // Source drainer
	group   sync.WaitGroup // compression gophers

	// Compressed frames
	lock    sync.Mutex
	frames  []*Frame  // Circular buffer of compressed frames
	running bool      // true if there are gophers running
	changed sync.Cond // New compressed frame available
}

// New compression pool
func New(rawFrames int, jpegFrames int, subsampling Subsampling, quality int, flags int) *Pool {
	pool := &Pool{
		subsampling: subsampling,
		quality:     quality,
		flags:       flags,
	}
	pool.changed.L = &(pool.lock)
	pool.drainer.Setup(rawFrames)

	pool.frames = make([]*Frame, jpegFrames)
	for i := 0; i < jpegFrames; i++ {
		pool.frames[i] = &Frame{}
	}
	return pool
}

// Stream the source provided, using a pool of `poolSize` compressors
func (pool *Pool) Stream(ctx context.Context, source Source, features RawFeatures, poolSize int) {
	// Make sure other sources are stopped
	pool.Wait()
	pool.drainer.Reset(0)
	// Clean the frames
	pool.lock.Lock()
	defer pool.lock.Unlock()
	for i := 0; i < len(pool.frames); i++ {
		pool.frames[i].number = 0
	}
	// launch the compressors
	tasks := make(chan task, pool.drainer.NumRawFrames)
	for i := 0; i < poolSize; i++ {
		pool.group.Add(1)
		go func() {
			defer pool.group.Done()
			compressor := NewCompressor()
			defer func() {
				compressor.Free()
			}()
			for task := range tasks {
				compressor = pool.run(compressor, task, features)
			}
		}()
	}
	// launch the source drainer
	pool.group.Add(1) // count the source also
	go func() {
		// Once the source drain is cancelled, clean it all up
		defer pool.group.Done()
		defer close(tasks)
		defer func() {
			pool.lock.Lock()
			defer pool.lock.Unlock()
			pool.running = false
			pool.changed.Broadcast()
		}()
		pool.drainer.Drain(ctx, source, tasks)
	}()
	// Set the running flag
	pool.running = true
}

// Wait for the pool to be fully stopped (it should be cancelled first)
func (pool *Pool) Wait() {
	pool.group.Wait()
	// Once no gophers are running, we can iterate the frames array
	for i := 0; i < len(pool.frames); i++ {
		pool.frames[i].group.Wait()
	}
}

// lock a frame for compressing
func (pool *Pool) lockFrame(frameNumber uint64) (frameIndex int, frame *Frame) {
	frameIndex = int(frameNumber % uint64(len(pool.frames)))
	pool.lock.Lock()
	defer pool.lock.Unlock()
	frame = pool.frames[frameIndex]
	frame.number = frameNumber
	frame.status = FrameCompressing
	return
}

// release a frame after compression
func (pool *Pool) releaseFrame(frameIndex int, status FrameStatus) {
	pool.lock.Lock()
	defer pool.lock.Unlock()
	pool.frames[frameIndex].status = status
	pool.changed.Broadcast()
}

// run a compression task
func (pool *Pool) run(compressor Compressor, task task, features RawFeatures) Compressor {
	// Release raw frame on ending
	defer func() {
		pool.drainer.Release(task.rawFrame)
	}()
	frameIndex, frame := pool.lockFrame(task.frameNumber)
	status := FrameFailed
	// Release compressed frame on ending
	defer func() {
		pool.releaseFrame(frameIndex, status)
	}()
	// Make sure the frame is not stuck sending somewhere
	unused := make(chan struct{})
	go func() {
		defer close(unused)
		frame.group.Wait() // wait until no readers, since we might overwrite Image internal buffers
	}()
	watchdog := time.NewTimer(time.Second)
	select {
	case <-unused: // everything fine
		watchdog.Stop()
	case <-watchdog.C:
		status = FrameStuck
		return compressor
	}
	// Once the frame is unused, overwrite it
	var err error
	frame.features, err = compressor.Compress(task.rawFrame, features, &(frame.image), pool.subsampling, pool.quality, pool.flags)
	if err != nil {
		// Release the image buffer, just in case
		frame.image.Free()
		// Also release the compressor
		compressor.Free()
		compressor = NewCompressor()
	} else {
		status = FrameReady
	}
	return compressor
}

// Next returns the frame with the given number or closer to that, or nil if not streaming
func (pool *Pool) Next(frameNumber uint64) (*Frame, uint64, FrameStatus) {
	pool.lock.Lock()
	defer pool.lock.Unlock()
	for {
		currentFrame := pool.drainer.CurrentFrame()
		frameIndex := int(frameNumber % uint64(len(pool.frames)))
		frame := pool.frames[frameIndex]
		switch {
		case !pool.running:
			// Returns nil on cancelled
			return nil, 0, FrameEmpty
		case frameNumber == frame.number:
			if frame.status != FrameEmpty && frame.status != FrameCompressing {
				frame.group.Add(1) // Increase the number of readers of the frame
				return frame, frame.number, frame.status
			}
			pool.changed.Wait()
		case frameNumber > frame.number:
			if frameNumber > currentFrame { // if waiting for a frame yet to arrive, set it to the next frame
				frameNumber = currentFrame + 1
			}
			pool.changed.Wait()
		case frameNumber < frame.number:
			// If frame is way too old, skip to the oldest frame we might still have
			if frameNumber+uint64(len(pool.frames)) <= currentFrame {
				frameNumber = currentFrame - uint64(len(pool.frames))
			}
			frameNumber += 1
		}
	}
}

// Release frame so that the buffer might be used again
func (frame *Frame) Release() {
	frame.group.Done()
}

// Return the frame content as a byte array.
// Must not be used after Release
func (frame *Frame) Slice() []byte {
	return frame.image.Slice()
}
