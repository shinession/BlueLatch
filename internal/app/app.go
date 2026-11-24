// Package app is what the window shows and does: the list of paired audio
// devices, kept fresh, and at most one connection being retried. It is the layer
// between the window and the retry engine. The window only reads Snapshot and
// calls Connect and Cancel; it has no retry logic of its own.
package app

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/shinession/bluelatch/internal/bt"
	"github.com/shinession/bluelatch/internal/latch"
)

var (
	// ErrBusy is returned by Connect while another device is being connected.
	ErrBusy = errors.New("another device is being connected")

	// ErrUnknownDevice is returned by Connect for a device that is not in the list.
	ErrUnknownDevice = errors.New("the device is not in the list of paired audio devices")

	// ErrClosing is returned by Connect once Shutdown has been called.
	ErrClosing = errors.New("the program is closing")
)

// Phases of a job, as the window sees them.
const (
	PhaseStarting     = "starting"
	PhaseReconnecting = "reconnecting"
	PhaseWaiting      = "waiting"
	PhaseBackoff      = "backoff"
	PhaseCancelling   = "cancelling"
	PhaseConnected    = "connected"
	PhaseCancelled    = "cancelled"
	PhaseFailed       = "failed"
)

// State is everything the window shows. It is rendered as JSON.
type State struct {
	Version string       `json:"version"`
	Devices []DeviceView `json:"devices"`
	Problem *Problem     `json:"problem,omitempty"` // the devices cannot be read
	Job     *JobView     `json:"job,omitempty"`     // the connection being retried, or the last one
	Closing bool         `json:"closing"`
}

// DeviceView is one paired audio device.
type DeviceView struct {
	Name      string `json:"name"`
	Address   string `json:"address"`
	Kind      string `json:"kind"` // headphones, headset, speaker or audio
	Connected bool   `json:"connected"`
}

// Problem says why the list of devices could not be read.
type Problem struct {
	Code string `json:"code"`           // no-radio or error
	Text string `json:"text,omitempty"` // what Windows said, for error
}

// JobView is a connection that is being retried, or that has just ended.
type JobView struct {
	Address string `json:"address"`
	Name    string `json:"name"`
	Phase   string `json:"phase"`
	Attempt int    `json:"attempt"`

	StartedAt int64 `json:"startedAt"`        // Unix milliseconds
	NextAt    int64 `json:"nextAt,omitempty"` // backoff: when the next attempt starts
	DoneAt    int64 `json:"doneAt,omitempty"` // when it ended
	Elapsed   int64 `json:"elapsed,omitempty"`

	Detail    string `json:"detail,omitempty"`    // what Windows did with each service
	Error     string `json:"error,omitempty"`     // why the last attempt, or the job, failed
	ErrorCode string `json:"errorCode,omitempty"` // no-radio, not-paired, no-services or gave-up
	Done      bool   `json:"done"`
}

// Config tunes an App.
type Config struct {
	Adapter bt.Adapter
	Policy  latch.Policy
	Log     *slog.Logger
	Version string

	// Refresh is how often the list of devices is read. Zero means 2 seconds.
	Refresh time.Duration
}

// App keeps the list of devices fresh and runs the connection job. All methods
// may be called from any goroutine.
type App struct {
	cfg Config
	log *slog.Logger
	now func() time.Time

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	kick   chan struct{}

	mu          sync.Mutex
	devices     []DeviceView
	problem     *Problem
	lastProblem string
	job         *job
	closing     bool
}

type job struct {
	view       JobView
	cancelling bool
	cancel     context.CancelFunc
}

// New returns an App. Call Start to begin reading the devices.
func New(cfg Config) *App {
	if cfg.Log == nil {
		cfg.Log = slog.New(slog.DiscardHandler)
	}
	if cfg.Refresh <= 0 {
		cfg.Refresh = 2 * time.Second
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &App{
		cfg:     cfg,
		log:     cfg.Log,
		now:     time.Now,
		ctx:     ctx,
		cancel:  cancel,
		kick:    make(chan struct{}, 1),
		devices: []DeviceView{},
	}
}

// Start reads the devices once, so the first Snapshot is complete, and then keeps
// reading them in the background until Shutdown.
func (a *App) Start() {
	a.refresh()
	a.wg.Add(1)
	go a.poll()
}

// Snapshot returns what the window shows now.
func (a *App) Snapshot() State {
	a.mu.Lock()
	defer a.mu.Unlock()
	s := State{
		Version: a.cfg.Version,
		Devices: append([]DeviceView{}, a.devices...),
		Problem: a.problem,
		Closing: a.closing,
	}
	if a.job != nil {
		v := a.job.view
		s.Job = &v
	}
	return s
}

// Busy reports whether a connection is being retried.
func (a *App) Busy() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.job != nil && !a.job.view.Done
}

// Connect starts retrying the connection of the device with that address. It
// returns at once; Snapshot shows the progress.
func (a *App) Connect(address string) error {
	addr, err := bt.ParseAddress(address)
	if err != nil {
		return err
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closing {
		return ErrClosing
	}
	if a.job != nil && !a.job.view.Done {
		return ErrBusy
	}
	var dev *DeviceView
	for i := range a.devices {
		if a.devices[i].Address == addr.String() {
			dev = &a.devices[i]
			break
		}
	}
	if dev == nil {
		return ErrUnknownDevice
	}

	ctx, cancel := context.WithCancel(a.ctx)
	j := &job{
		cancel: cancel,
		view: JobView{
			Address:   dev.Address,
			Name:      dev.Name,
			Phase:     PhaseStarting,
			StartedAt: a.now().UnixMilli(),
		},
	}
	a.job = j
	a.wg.Add(1)
	go a.run(ctx, j, addr)
	return nil
}

// Cancel stops the connection that is being retried. A request that Windows is
// working on is not interrupted, so the job ends when that request has returned.
func (a *App) Cancel() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cancelLocked()
}

func (a *App) cancelLocked() {
	if a.job == nil || a.job.view.Done || a.job.cancelling {
		return
	}
	a.job.cancelling = true
	a.job.view.Phase = PhaseCancelling
	a.job.view.NextAt = 0
	a.job.cancel()
}

// Shutdown stops the connection that is being retried and the reading of the
// devices, and waits for a request that Windows is working on to return, because
// stopping in the middle of it could leave a service switched off. It reports
// false when that took longer than timeout. It may be called more than once.
func (a *App) Shutdown(timeout time.Duration) bool {
	a.mu.Lock()
	a.closing = true
	a.cancelLocked()
	a.mu.Unlock()
	a.cancel()

	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}

// RefreshNow asks for the devices to be read as soon as possible.
func (a *App) RefreshNow() {
	select {
	case a.kick <- struct{}{}:
	default:
	}
}

func (a *App) poll() {
	defer a.wg.Done()
	ticker := time.NewTicker(a.cfg.Refresh)
	defer ticker.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
		case <-a.kick:
		}
		a.refresh()
	}
}

// refresh reads the devices and keeps the paired audio ones.
func (a *App) refresh() {
	devices, err := a.cfg.Adapter.Devices()

	views := []DeviceView{}
	var problem *Problem
	switch {
	case errors.Is(err, bt.ErrNoRadio):
		problem = &Problem{Code: "no-radio"}
	case err != nil:
		problem = &Problem{Code: "error", Text: err.Error()}
	default:
		for _, d := range devices {
			if d.Paired() && d.IsAudio() {
				views = append(views, DeviceView{
					Name:      d.DisplayName(),
					Address:   d.Address.String(),
					Kind:      kindOf(d.Class),
					Connected: d.Connected,
				})
			}
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.devices = views
	a.problem = problem
	text := ""
	if problem != nil {
		text = problem.Code + ": " + problem.Text
	}
	if text != a.lastProblem { // log a change, not every read
		a.lastProblem = text
		if problem != nil {
			a.log.Warn("the devices cannot be read", "problem", text)
		} else {
			a.log.Info("the devices can be read again")
		}
	}
}

// run retries one connection until it ends.
func (a *App) run(ctx context.Context, j *job, addr bt.Address) {
	defer a.wg.Done()
	engine := latch.New(a.cfg.Adapter, a.cfg.Policy, a.log)
	res, err := engine.Connect(ctx, addr, func(p latch.Progress) { a.progress(j, p) })
	a.finish(j, res, err)
	a.RefreshNow()
}

// progress records what the engine reports.
func (a *App) progress(j *job, p latch.Progress) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if j.cancelling {
		return // the window keeps showing that the job is being stopped
	}
	v := &j.view
	v.Attempt = p.Attempt
	v.Detail = p.Detail
	v.Error = ""
	if p.Err != nil {
		v.Error = p.Err.Error()
	}
	v.NextAt = 0
	switch p.Phase {
	case latch.PhaseReconnecting:
		v.Phase = PhaseReconnecting
	case latch.PhaseWaiting:
		v.Phase = PhaseWaiting
	case latch.PhaseBackoff:
		v.Phase = PhaseBackoff
		v.NextAt = a.now().Add(p.Delay).UnixMilli()
	}
}

// finish records how the job ended.
func (a *App) finish(j *job, res latch.Result, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	v := &j.view
	v.Done = true
	v.DoneAt = a.now().UnixMilli()
	v.Attempt = res.Attempts
	v.Elapsed = res.Elapsed.Milliseconds()
	v.NextAt = 0
	v.Error, v.ErrorCode = "", ""
	switch {
	case err == nil:
		v.Phase = PhaseConnected
		v.Detail = ""
	case errors.Is(err, context.Canceled):
		v.Phase = PhaseCancelled
	default:
		v.Phase = PhaseFailed
		v.Error = err.Error()
		v.ErrorCode = errorCode(err)
	}
}

func errorCode(err error) string {
	switch {
	case errors.Is(err, bt.ErrNoRadio):
		return "no-radio"
	case errors.Is(err, bt.ErrDeviceNotFound):
		return "not-paired"
	case errors.Is(err, bt.ErrNoAudioServices):
		return "no-services"
	case errors.Is(err, latch.ErrGaveUp):
		return "gave-up"
	}
	return ""
}

// kindOf says what kind of audio device a class of device is, so the window can
// pick an icon.
func kindOf(c bt.ClassOfDevice) string {
	if !c.IsAudioVideo() {
		return "audio"
	}
	switch c.Minor() {
	case 0x01, 0x02: // wearable headset, hands-free
		return "headset"
	case 0x06:
		return "headphones"
	case 0x05, 0x0a, 0x0f: // loudspeaker, hi-fi audio, video display and loudspeaker
		return "speaker"
	}
	return "audio"
}
