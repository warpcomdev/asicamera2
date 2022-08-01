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
	Next(ctx context.Context, img *Image) error // get next frame
}

// --------------------------------
// Metrics
// --------------------------------

var (
	compressionTime = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "compression_latency",
		Help: "JPEG Compression latency",
		Buckets: []float64{
			10, 30, 60, 120, 250, 500, 1000, 2500,
		},
	})

	compressedSize = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "compressed_size",
		Help: "Size of compressed frames",
		Buckets: []float64{
			16384, 65535, 262144, 524288, 1048576, 2097152, 4194304,
		},
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
// Raw Pool - implements a fixed pool of buffers
// for raw images.
// ---------------------------------

// rawPool manages raw image frames
type rawPool struct {
	freeList chan *Image
	poolSize int
}

// Setup the stream with initial buffers
func newRawPool(poolSize int, features RawFeatures) *rawPool {
	pool := &rawPool{
		poolSize: poolSize,
		freeList: make(chan *Image, poolSize),
	}
	for i := 0; i < poolSize; i++ {
		image := &Image{}
		image.Alloc(features.Pitch() * features.Height)
		pool.freeList <- image
	}
	return pool
}

// Stream frame
type rawFrame struct {
	number   uint64
	image    *Image      // input buffer
	features RawFeatures // raw frame features
}

// Stream a source through a rawFrame channel
func (pool *rawPool) stream(ctx context.Context, source Source, features RawFeatures) chan rawFrame {
	frames := make(chan rawFrame, pool.poolSize)
	go func() {
		defer close(frames)
		var frameNumber uint64 = 1 // always skip frame 0
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
			if err := source.Next(ctx, srcImage); err != nil {
				pool.freeList <- srcImage // return the image to the free list
				return
			}
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
				frameNumber += 1
			}
		}
	}()
	return frames
}

// Free all the resources of the pool
func (pool *rawPool) Free() {
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
	FrameFailed                         // Frame failed to ocmpress
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
type JpegFrame struct {
	number   uint64
	image    Image
	features JpegFeatures
	status   FrameStatus
	group    sync.WaitGroup // readers sending the image to clients
}

// Done tells the compressors they can overwrite the internal buffers
func (frame *JpegFrame) Done() {
	frame.group.Done()
}

// Return the frame content as a byte array.
// Must not be used after Done
func (frame *JpegFrame) Slice() []byte {
	return frame.image.Slice()
}

// ----------------------------
// JpegPool: hash buffer for compressed images.
// Hash key is just the frame number - the pool is designed to store
// consecutive frames.
// The pool size must be larger than the RawPool buffer size,
// to avoid overwriting a frame which is still being compressed.
// ------------------------------

// JpegPool holds a fixed size hash of compressed frames
type jpegPool struct {
	sync.Mutex
	sync.Cond
	frames []*JpegFrame // Buffer of compressed frames
}

// New compressed pool
func newJpegPool(poolSize int, features RawFeatures) *jpegPool {
	pool := &jpegPool{
		frames: make([]*JpegFrame, poolSize),
	}
	pool.Cond.L = &(pool.Mutex)
	for i := 0; i < poolSize; i++ {
		frame := &JpegFrame{}
		frame.image.Alloc(features.Pitch() * features.Height)
		pool.frames[i] = frame
	}
	return pool
}

// Join waits for all readers to finish and frees compressed pool resources
func (p *jpegPool) Join() {
	for i := 0; i < len(p.frames); i++ {
		p.frames[i].group.Wait()
		p.frames[i].image.Free()
	}
}

// lock a frame for compressing
func (pool *jpegPool) hold(frameNumber uint64) (frameIndex int, frame *JpegFrame, oldStatus FrameStatus) {
	frame, frameIndex = pool.frameAt(frameNumber)
	pool.Lock()
	defer pool.Unlock()
	oldStatus = frame.status
	frame.number = frameNumber
	frame.status = FrameCompressing
	return
}

// release a frame after compression
func (pool *jpegPool) release(frameIndex int, status FrameStatus) {
	pool.Lock()
	defer pool.Unlock()
	pool.frames[frameIndex].status = status
	pool.Broadcast()
}

// frameAt returns the frame and frameIndex for the given frameNumber
func (pool *jpegPool) frameAt(frameNumber uint64) (*JpegFrame, int) {
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
	jpegPool *jpegPool       // Pool to use for compressed frames
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
func newFarm(farmSize, taskSize int, subsampling Subsampling, quality int, flags int) *Farm {
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
	// Get the buffer for compressed frame, release at the end
	frameIndex, frame, oldStatus := task.jpegPool.hold(task.rawFrame.number)
	newStatus := FrameFailed
	defer func() {
		statusName := frameStatusNames[newStatus]
		compressionStatus.WithLabelValues(statusName).Inc()
	}()
	defer func() {
		task.jpegPool.release(frameIndex, newStatus)
	}()
	if oldStatus == FrameCompressing {
		// Someone else compressing the frame, should not happen.
		return compressor
	}
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
		newStatus = FrameStuck
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

// ------------------------------
// Pipeline: holds the frame pools (both raw and compressed),
// and the compression farm
// ------------------------------

type Pipeline struct {
	rawPool  *rawPool
	jpegPool *jpegPool
	farm     *Farm
}

// New compressin pipeline
// jpegPoolSize must be > rawPoolSize + farmSize`
func New(rawPoolSize, jpegPoolSize, farmSize int, features RawFeatures, subsampling Subsampling, quality int, flags int) *Pipeline {
	return &Pipeline{
		rawPool:  newRawPool(rawPoolSize, features),
		jpegPool: newJpegPool(jpegPoolSize, features),
		farm:     newFarm(farmSize, rawPoolSize, subsampling, quality, flags),
	}
}

// Join and free all resources. All Sessions must be joined before this.
func (p *Pipeline) Join() {
	p.farm.Stop()
	p.jpegPool.Join()
	p.rawPool.Free()
}

// Session keeps streaming from the Source until cancelled
type Session struct {
	currentFrame  uint64 // latest frame sent for compression. Not running if 0.
	pendingFrames sync.WaitGroup
	jpegPool      *jpegPool
}

// Session starts a streaming session from the given Source
func (pipeline *Pipeline) Session(ctx context.Context, source Source, features RawFeatures) *Session {
	session := &Session{
		currentFrame: 1, // 0 is reserved for closed stream
		jpegPool:     pipeline.jpegPool,
	}
	session.pendingFrames.Add(1)
	start := time.Now()
	go func() {
		defer func() {
			sessionDuration.Observe(time.Since(start).Seconds())
		}()
		defer session.pendingFrames.Done()
		defer func() {
			// Store frmae number 0 -> not running
			atomic.StoreUint64(&(session.currentFrame), 0)
			session.jpegPool.Broadcast() // let all the readers notice
		}()
		for rawFrame := range pipeline.rawPool.stream(ctx, source, features) {
			rawFrame := rawFrame // avoid aliasing the loop variable
			atomic.StoreUint64(&(session.currentFrame), rawFrame.number)
			session.pendingFrames.Add(1) // Will be flagged .Done() by compressor
			pipeline.farm.push(farmTask{
				rawFrame: rawFrame,
				freeList: pipeline.rawPool.freeList,
				jpegPool: pipeline.jpegPool,
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
func (session *Session) Next(frameNumber uint64) (*JpegFrame, uint64, FrameStatus) {
	session.jpegPool.Lock()
	defer session.jpegPool.Unlock()
	for {
		frame, _ := session.jpegPool.frameAt(frameNumber)
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
		session.jpegPool.Wait()
	}
}
