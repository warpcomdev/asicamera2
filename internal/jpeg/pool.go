package jpeg

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Source of frames
type Source interface {
	Next(ctx context.Context, img *Image) (uint64, error) // get next frame
}

// --------------------------------
// Metrics
// --------------------------------

var (
	compressionTime = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "compression_time",
		Help: "JPEG Compression time (milliseconds)",
		Buckets: []float64{
			10, 30, 60, 120, 250, 500, 1000, 2500,
		},
	})

	compressedSize = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "compressed_size",
		Help: "Size of compressed frames (bytes)",
		Buckets: []float64{
			16384, 65535, 262144, 524288, 1048576, 2097152, 4194304,
		},
	})

	skippedFrames = promauto.NewCounter(prometheus.CounterOpts{
		Name: "skipped_frames",
		Help: "Number of frames skipped",
	})

	compressionStatus = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "compression_status",
		Help: "Compression results by status",
	}, []string{"status"})

	streamingSessions = promauto.NewCounter(prometheus.CounterOpts{
		Name: "pipeline_sessions",
		Help: "Accumulated number of sessions",
	})

	sessionDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "pipeline_session_duration",
		Help: "Pipeline session duration (seconds)",
		Buckets: []float64{
			1, 60, 1800, 7200, 28800,
		},
	})
)

// ---------------------------------
// Pool - implements a fixed pool of buffers
// for raw images.
// ---------------------------------

// Pool manages raw image frames
type Pool struct {
	freeList chan *Image
	poolSize int
	free     chan struct{} // closed by Free to shutdown the watchdog
}

// Setup the stream with initial buffers
func NewPool(poolSize int, features RawFeatures) *Pool {
	pool := &Pool{
		poolSize: poolSize,
		freeList: make(chan *Image, poolSize),
		free:     make(chan struct{}),
	}
	for i := 0; i < poolSize; i++ {
		image := &Image{}
		image.Alloc(features.Pitch() * features.Height)
		pool.freeList <- image
	}
	go pool.watchdog()
	return pool
}

// Stream frame
type rawFrame struct {
	number   uint64
	image    *Image      // input buffer
	features RawFeatures // raw frame features
}

// Stream a source through a rawFrame channel
func (pool *Pool) stream(ctx context.Context, source Source, features RawFeatures) chan rawFrame {
	frames := make(chan rawFrame, pool.poolSize)
	var lastFrame uint64
	go func() {
		defer close(frames)
		for {
			var srcImage *Image
			var ok bool
			select {
			case <-ctx.Done():
				return
			case srcImage, ok = <-pool.freeList:
				if !ok {
					return
				}
			}
			frameNumber, err := source.Next(ctx, srcImage)
			if err != nil || frameNumber == 0 {
				pool.freeList <- srcImage // return the image to the free list
				return
			}
			if lastFrame > 0 {
				frameDiff := frameNumber - lastFrame
				if frameDiff > 1 {
					skippedFrames.Add(float64(frameDiff - 1))
				}
			}
			lastFrame = frameNumber
			newFrame := rawFrame{
				number:   frameNumber,
				image:    srcImage,
				features: features,
			}
			select {
			case <-ctx.Done():
				pool.freeList <- srcImage
				return
			case frames <- newFrame:
			}
		}
	}()
	return frames
}

// Monitors the free list
func (pool *Pool) watchdog() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		// align with a tick interval
		select {
		case <-pool.free:
			return
		case <-ticker.C:
		}
		// try to acquire a free buffer before the next tick
		select {
		case <-pool.free:
			return
		case img, ok := <-pool.freeList:
			if !ok {
				return // list closed
			}
			pool.freeList <- img
		case <-ticker.C:
			panic("no free raw buffers for 10 seconds")
		}
	}
}

// Free all the resources of the pool
func (pool *Pool) Free() {
	close(pool.free) // stop the watchdog
	// consume all freeList items
	for i := 0; i < pool.poolSize; i++ {
		frame := <-pool.freeList
		frame.Free()
	}
	close(pool.freeList)
}

// -------------------------
// Frame: contains info about a compressed image,
// along with the compression status
// -------------------------

type FrameStatus int

const (
	FrameEmpty       FrameStatus = iota // Frame has no valid data
	FrameCompressing                    // Frame is being compressed
	FrameReady                          // Frame is ready to stream
	FrameFailed                         // Frame failed to compress
	FrameStuck                          // frame was not processed because some reader had it locked
)

var frameStatusNames = []string{
	"FrameEmpty",
	"FrameCompressing",
	"FrameReady",
	"FrameFailed",
	"FrameStuck",
}

// Compressed frame
type Frame struct {
	number   uint64
	image    Image
	features JpegFeatures
	status   FrameStatus
	group    sync.WaitGroup // readers sending the image to clients
}

// Done tells the compressors they can overwrite the internal buffers
func (frame *Frame) Done() {
	frame.group.Done()
}

// Return the frame content as a byte array.
// Must not be used after Done
func (frame *Frame) Slice() []byte {
	return frame.image.Slice()
}

// Wait for the frame to be available (nothing in the wait group)
func (frame *Frame) available(t time.Duration) bool {
	available := make(chan struct{})
	go func() {
		defer close(available)
		frame.group.Wait() // wait until no readers
		// The goroutine will leak if there are readers stuck.
		// But JpegPool will trigger a watchdog that will panic
		// if the frame is stuck for too long, so this is ok.
	}()
	watchdog := time.NewTimer(t)
	select {
	case <-available: // everything fine
		watchdog.Stop()
		return true
	case <-watchdog.C:
		return false
	}
}

// ----------------------------
// Buffer: hash buffer for compressed images.
// Hash key is just the frame number - the pool is designed to store
// consecutive frames.
// The pool size must be larger than the RawPool buffer size,
// to avoid overwriting a frame which is still being compressed.
// ------------------------------

// Buffer holds a fixed size hash of compressed frames
type Buffer struct {
	sync.Mutex
	sync.Cond
	frames []*Frame // Buffer of compressed frames
}

// New compressed buffer
func NewBuffer(poolSize int, features RawFeatures) *Buffer {
	pool := &Buffer{
		frames: make([]*Frame, poolSize),
	}
	pool.Cond.L = &(pool.Mutex)
	for i := 0; i < poolSize; i++ {
		frame := &Frame{}
		frame.image.Alloc(features.Pitch() * features.Height)
		pool.frames[i] = frame
	}
	return pool
}

// Join waits for all readers to finish and frees compressed pool resources
func (p *Buffer) Join() {
	for i := 0; i < len(p.frames); i++ {
		p.frames[i].group.Wait()
		p.frames[i].image.Free()
	}
}

// lock a frame for compressing.
func (pool *Buffer) hold(frameNumber uint64) (frameIndex int, frame *Frame, oldStatus FrameStatus) {
	pool.Lock()
	defer pool.Unlock()
	frame, frameIndex = pool.frameAt(frameNumber)
	oldStatus = frame.status
	if oldStatus == FrameCompressing {
		return 0, nil, oldStatus // should not happen
	}
	frame.number = frameNumber
	frame.status = FrameCompressing
	return
}

// release a frame after compression.
func (pool *Buffer) release(frameIndex int, status FrameStatus) {
	pool.Lock()
	defer pool.Unlock()
	pool.frames[frameIndex].status = status
	pool.Broadcast()
}

// frameAt returns the frame and frameIndex for the given frameNumber
func (pool *Buffer) frameAt(frameNumber uint64) (*Frame, int) {
	frameIndex := int(frameNumber % uint64(len(pool.frames)))
	return pool.frames[frameIndex], frameIndex
}

// ---------------------------
// Farm: fixed set of compressors
// ---------------------------

// Task running in a compression farm
type farmTask struct {
	rawFrame rawFrame        // Input frame to compress
	freeList chan *Image     // Return raw image to free list when done
	buffer   *Buffer         // Buffer to use for compressed frames
	group    *sync.WaitGroup // notify on compression finished
}

// Farm  of compression gophers
type Farm struct {
	// Compression parameters
	subsampling Subsampling
	quality     int
	flags       int

	// Compression gopher group
	tasks chan farmTask
	group sync.WaitGroup
}

// New compression pool
func NewFarm(farmSize, taskSize int, subsampling Subsampling, quality int, flags int) *Farm {
	farm := &Farm{
		subsampling: subsampling,
		quality:     quality,
		flags:       flags,
		tasks:       make(chan farmTask, taskSize),
	}
	// launch the compressors
	for i := 0; i < farmSize; i++ {
		farm.group.Add(1)
		go func() {
			defer farm.group.Done()
			compressor := NewCompressor()
			defer func() {
				compressor.Free()
			}()
			for task := range farm.tasks {
				start := time.Now()
				compressor = farm.run(compressor, task)
				compressionTime.Observe(float64(time.Since(start).Milliseconds()))
			}
		}()
	}
	return farm
}

// Stop closes the task channel and stops accepting tasks
func (farm *Farm) Stop() {
	close(farm.tasks)
	farm.group.Wait()
}

// Push a task to the pool. Panics if called after Stop()
func (farm *Farm) push(task farmTask) {
	farm.tasks <- task
}

// run a compression task
func (farm *Farm) run(compressor Compressor, task farmTask) Compressor {
	// Notify the task and release resources at the end
	defer func() {
		task.freeList <- task.rawFrame.image
		task.group.Done()
	}()
	// Get the buffer for compressed frame
	frameIndex, frame, oldStatus := task.buffer.hold(task.rawFrame.number)
	if frame == nil {
		// Someone else compressing the frame, should not happen.
		return compressor
	}
	// Prepare the cleanup functions
	newStatus := FrameFailed
	defer func() {
		statusName := frameStatusNames[newStatus]
		compressionStatus.WithLabelValues(statusName).Inc()
	}()
	defer func() {
		task.buffer.release(frameIndex, newStatus)
	}()
	// Make sure the frame is not stuck sending somewhere
	if !frame.available(2 * time.Second) {
		newStatus = FrameStuck
		if oldStatus != FrameStuck {
			// Start a watchdog if the frame was not stuck before
			go func() {
				if !frame.available(10 * time.Second) {
					panic("compressed frame stuck for longer than 10 seconds")
				}
			}()
		}
		return compressor
	}
	// Once the frame is unused, overwrite it
	var err error
	frame.features, err = compressor.Compress(
		task.rawFrame.image,
		task.rawFrame.features,
		&(frame.image),
		farm.subsampling,
		farm.quality,
		farm.flags|TJFLAG_NOREALLOC,
	)
	if err != nil {
		// Release the compressor, just in case
		compressor.Free()
		compressor = NewCompressor()
	} else {
		newStatus = FrameReady
		compressedSize.Observe(float64(frame.image.Size()))
	}
	return compressor
}

// Session keeps streaming from the Source until cancelled
type Session struct {
	currentFrame  uint64 // latest frame sent for compression. Not running if 0.
	pendingFrames sync.WaitGroup
	buffer        *Buffer
}

// Session starts a streaming session from the given Source
func (buffer *Buffer) Session(ctx context.Context, source Source, features RawFeatures, pool *Pool, farm *Farm) *Session {
	session := &Session{
		currentFrame: 1, // 0 is reserved for closed stream
		buffer:       buffer,
	}
	session.pendingFrames.Add(1)
	start := time.Now()
	go func() {
		defer func() {
			sessionDuration.Observe(time.Since(start).Seconds())
		}()
		defer session.pendingFrames.Done()
		defer func() {
			// Store frame number 0 -> not running
			atomic.StoreUint64(&(session.currentFrame), 0)
			session.buffer.Broadcast() // let all the readers notice
		}()
		for rawFrame := range pool.stream(ctx, source, features) {
			rawFrame := rawFrame // avoid aliasing the loop variable
			atomic.StoreUint64(&(session.currentFrame), rawFrame.number)
			session.pendingFrames.Add(1) // Will be flagged .Done() by compressor
			farm.push(farmTask{
				rawFrame: rawFrame,
				freeList: pool.freeList,
				buffer:   buffer,
				group:    &(session.pendingFrames),
			})
		}
	}()
	streamingSessions.Inc()
	return session
}

// Current frame number. If == 0, session is stopped.
func (session *Session) CurrentFrame() uint64 {
	return atomic.LoadUint64(&(session.currentFrame))
}

// Join the session once it has been cancelled
func (session *Session) Join() {
	session.pendingFrames.Wait()
}

// Next frame at or after the given frameNumber for this session.
// Returns nil if the stream is closed
func (session *Session) Next(frameNumber uint64) (*Frame, uint64, FrameStatus) {
	session.buffer.Lock()
	defer session.buffer.Unlock()
	for {
		frame, _ := session.buffer.frameAt(frameNumber)
		currentFrame := session.CurrentFrame()
		switch {
		case currentFrame == 0: // stopped
			return nil, 0, FrameEmpty
		case frame.number == frameNumber: // frame in the pool
			if frame.status != FrameEmpty && frame.status != FrameCompressing {
				frame.group.Add(1) // Increment the read count for this frame
				return frame, frame.number, frame.status
			}
		case frameNumber > currentFrame: // future frame
			frameNumber = currentFrame + 1
		default:
			// The frame number is past and no longer available.
			// pick the current frame.
			frameNumber = currentFrame
		}
		session.buffer.Wait()
	}
}
