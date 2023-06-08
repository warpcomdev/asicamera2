package jpeg

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

// --------------------------------
// Metrics
// --------------------------------

var (
	compressionLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "compression_latency",
			Help: "JPEG Compression latency",
			Buckets: []float64{
				10, 30, 60, 120, 250, 500, 1000, 2500,
			},
		},
		[]string{"camera"},
	)

	streamingSessions = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pipeline_sessions",
			Help: "Accumulated number of sessions",
		},
		[]string{"camera"},
	)

	sessionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "pipeline_session_duration",
			Help: "Pipeline session duration (seconds)",
			Buckets: []float64{
				1, 60, 1800, 7200, 28800,
			},
		},
		[]string{"camera"},
	)

	compressionStatus = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "compression_status",
			Help: "Compression results by status",
		},
		[]string{"camera", "status"},
	)
)

// ---------------------------------
// Raw Pool - implements a fixed pool of buffers
// for raw images.
// ---------------------------------

// Pool manages raw image frames
type Pool struct {
	freeList chan *Image
	poolSize int
	free     chan struct{} // closed by Free to shutdown the watchdog
}

// Setup the stream with initial buffers
func NewPool(poolSize int, imgSize int) *Pool {
	pool := &Pool{
		poolSize: poolSize,
		freeList: make(chan *Image, poolSize),
		free:     make(chan struct{}),
	}
	for i := 0; i < poolSize; i++ {
		image := &Image{}
		image.Alloc(imgSize)
		pool.freeList <- image
	}
	go pool.watchdog()
	return pool
}

// Adds additional utility information to SrcFrame
type srcFrame struct {
	number    uint64
	camera    string
	timestamp time.Time
	SrcFrame
}

// Stream a source through a SrcFrame channel
func (pool *Pool) stream(ctx context.Context, logger *zap.Logger, source Source) chan srcFrame {
	frames := make(chan srcFrame, pool.poolSize)
	go func() {
		defer close(frames)
		var frameNumber uint64 = 1 // always skip frame 0
		cameraName := source.Name()
		for {
			var (
				srcImage *Image
				ok       bool
				err      error
			)
			select {
			case <-ctx.Done():
				return
			case srcImage, ok = <-pool.freeList:
				if !ok {
					return
				}
				break
			}
			newFrame := srcFrame{
				number: frameNumber,
				camera: cameraName,
			}
			newFrame.SrcFrame, err = source.Next(ctx, srcImage)
			if err != nil {
				logger.Error("Failed to get next frame", zap.Error(err))
				pool.freeList <- srcImage // return the image to the free list
				return
			}
			select {
			case <-ctx.Done():
				pool.freeList <- srcImage
				return
			case frames <- newFrame:
				frameNumber += 1
				break
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
			break
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
			break
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

// Wait for the frame to be available (nothing in the wait group)
func (frame *JpegFrame) available(t time.Duration) bool {
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
func newJpegPool(poolSize int, imageSize int) *jpegPool {
	pool := &jpegPool{
		frames: make([]*JpegFrame, poolSize),
	}
	pool.Cond.L = &(pool.Mutex)
	for i := 0; i < poolSize; i++ {
		frame := &JpegFrame{}
		frame.image.Alloc(imageSize)
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

// lock a frame for compressing.
func (pool *jpegPool) hold(frameNumber uint64) (frameIndex int, frame *JpegFrame, oldStatus FrameStatus) {
	frame, frameIndex = pool.frameAt(frameNumber)
	pool.Lock()
	defer pool.Unlock()
	oldStatus = frame.status
	if oldStatus == FrameCompressing {
		return 0, nil, oldStatus // should not happen
	}
	frame.number = frameNumber
	frame.status = FrameCompressing
	return
}

// release a frame after compression.
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
	rawFrame srcFrame        // Input frame to compress
	freeList chan *Image     // Return raw image to free list when done
	jpegPool *jpegPool       // Pool to use for compressed frames
	group    *sync.WaitGroup // notify on compression finished
}

// Farm  of compression gophers
type Farm struct {
	// Compression gopher group
	tasks chan farmTask
	group sync.WaitGroup
}

// New compression pool
func NewFarm(logger *zap.Logger, farmSize, taskSize int) *Farm {
	farm := &Farm{
		tasks: make(chan farmTask, taskSize),
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
				compressor = farm.run(logger, compressor, task)
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
func (farm *Farm) run(logger *zap.Logger, compressor Compressor, task farmTask) Compressor {
	// Notify the task and release resources at the end
	defer func() {
		task.freeList <- task.rawFrame.Buffer()
		task.group.Done()
	}()
	// Get the buffer for compressed frame
	frameIndex, frame, oldStatus := task.jpegPool.hold(task.rawFrame.number)
	if frame == nil {
		// Someone else compressing the frame, should not happen.
		return compressor
	}
	// Prepare the cleanup functions
	newStatus := FrameFailed
	defer func() {
		statusName := frameStatusNames[newStatus]
		compressionStatus.WithLabelValues(task.rawFrame.camera, statusName).Inc()
	}()
	defer func() {
		task.jpegPool.release(frameIndex, newStatus)
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
	frame.features, err = task.rawFrame.Compress(compressor, &frame.image)
	if err != nil {
		// Release the compressor, just in case
		logger.Error("Compression failed", zap.Error(err))
		compressor.Free()
		compressor = NewCompressor()
	} else {
		newStatus = FrameReady
		compressionLatency.WithLabelValues(task.rawFrame.camera).Observe(float64(time.Since(task.rawFrame.timestamp) / time.Second))
	}
	return compressor
}

// ------------------------------
// Pipeline: holds the frame pools (both raw and compressed),
// and the compression farm
// ------------------------------

type Pipeline struct {
	rawPool  *Pool
	jpegPool *jpegPool
	features RawFeatures
	farm     *Farm
}

// New creates a new compression pipeline.
// A pipeline is only valid for a given set of RawFeatures,
// because it must allocate buffers and the like.
// It is also valid for a single combination of subsampling,
// quality and flags, because images are encoded only once.
// jpegPoolSize must be > rawPoolSize + farmSize`
func New(pool *Pool, farm *Farm, jpegPoolSize, imageSize int) *Pipeline {
	return &Pipeline{
		rawPool:  pool,
		jpegPool: newJpegPool(jpegPoolSize, imageSize),
		farm:     farm,
	}
}

// Features returnts the Features with which the pipeline was created
func (p *Pipeline) Features() RawFeatures {
	return p.features
}

// Join and free all resources. All Sessions must be joined before this.
func (p *Pipeline) Join() {
	p.jpegPool.Join()
}

// Session keeps streaming from the Source until cancelled
type Session struct {
	currentFrame  uint64 // latest frame sent for compression. Not running if 0.
	pendingFrames sync.WaitGroup
	jpegPool      *jpegPool
}

// session starts a streaming session from the given Source.
// The source MUST BE compatible with the RawFeatures with which the
// pipeline was created.
func (pipeline *Pipeline) session(ctx context.Context, logger *zap.Logger, source Source) *Session {
	session := &Session{
		currentFrame: 1, // 0 is reserved for closed stream
		jpegPool:     pipeline.jpegPool,
	}
	session.pendingFrames.Add(1)
	start := time.Now()
	go func() {
		defer func() {
			sessionDuration.WithLabelValues(source.Name()).Observe(time.Since(start).Seconds())
		}()
		defer session.pendingFrames.Done()
		defer func() {
			// Store frame number 0 -> not running
			atomic.StoreUint64(&(session.currentFrame), 0)
			session.jpegPool.Broadcast() // let all the readers notice
		}()
		for rawFrame := range pipeline.rawPool.stream(ctx, logger, source) {
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
	streamingSessions.WithLabelValues(source.Name()).Inc()
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
