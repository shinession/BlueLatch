// Package app is what the window shows and does: the list of paired audio
// devices and the list of sound outputs, both kept fresh, at most one connection
// being retried, and at most one change of the sound output. It is the layer
// between the window and the retry engine. The window only reads Snapshot and
// calls Connect, Disconnect, Cancel and SwitchOutput; it has no retry logic of
// its own.
package app

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/shinession/bluelatch/internal/audio"
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

	// ErrUnknownOutput is returned by SwitchOutput for an output that is not in
	// the list.
	ErrUnknownOutput = errors.New("the sound output is not in the list")

	// ErrSwitching is returned by SwitchOutput while the output is being changed.
	ErrSwitching = errors.New("the sound output is being switched")
)

// The kinds of job: retrying a connection, and dropping one.
const (
	KindConnect    = "connect"
	KindDisconnect = "disconnect"
)

// Phases of a job, as the window sees them. A connection job goes through
// starting, reconnecting, waiting and backoff to connected; a disconnection job
// goes from disconnecting to disconnected. Both can end as cancelled or failed.
const (
	PhaseStarting      = "starting"
	PhaseReconnecting  = "reconnecting"
	PhaseWaiting       = "waiting"
	PhaseBackoff       = "backoff"
	PhaseCancelling    = "cancelling"
	PhaseConnected     = "connected"
	PhaseDisconnecting = "disconnecting"
	PhaseDisconnected  = "disconnected"
	PhaseCancelled     = "cancelled"
	PhaseFailed        = "failed"
)

// State is everything the window shows. It is rendered as JSON.
type State struct {
	Version string       `json:"version"`
	Devices []DeviceView `json:"devices"`
	Problem *Problem     `json:"problem,omitempty"` // the devices cannot be read
	Job     *JobView     `json:"job,omitempty"`     // the connection being retried, or the last one
	Sound   *SoundView   `json:"sound,omitempty"`   // the sound outputs; none without an audio adapter
	Closing bool         `json:"closing"`
}

// SoundView is the sound outputs of Windows.
type SoundView struct {
	Outputs []OutputView `json:"outputs"`
	Error   string       `json:"error,omitempty"`  // why the outputs cannot be read
	Change  *ChangeView  `json:"change,omitempty"` // the change being made, or the last one
}

// OutputView is one sound output.
type OutputView struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Kind    string `json:"kind"`    // speaker, headphones, headset, display or audio
	Current bool   `json:"current"` // the default output
}

// ChangeView is a change of the default output that is being made, or the one
// that has just ended.
type ChangeView struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Done      bool   `json:"done"`
	DoneAt    int64  `json:"doneAt,omitempty"` // Unix milliseconds
	Error     string `json:"error,omitempty"`
	ErrorCode string `json:"errorCode,omitempty"` // output-gone, not-switched or unsupported
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

// JobView is a connection that is being retried or a connection that is being
// dropped, or the one that has just ended.
type JobView struct {
	Kind    string `json:"kind"` // connect or disconnect
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
	ErrorCode string `json:"errorCode,omitempty"` // no-radio, not-paired, no-services, gave-up or still-connected
	Done      bool   `json:"done"`
}

// Config tunes an App.
type Config struct {
	Adapter bt.Adapter
	Audio   audio.Adapter // the sound outputs; nil leaves them out
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

	soundMu sync.Mutex // held while the outputs are read and stored, so an older reading never replaces a newer one

	mu          sync.Mutex
	devices     []DeviceView
	problem     *Problem
	lastProblem string
	job         *job
	outputs     []OutputView
	soundError  string
	change      *ChangeView
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

// Start reads the devices and the outputs once, so the first Snapshot is
// complete, and then keeps reading them in the background until Shutdown.
func (a *App) Start() {
	a.refresh()
	a.refreshSound()
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
	if a.cfg.Audio != nil {
		sound := &SoundView{Outputs: append([]OutputView{}, a.outputs...), Error: a.soundError}
		if a.change != nil {
			v := *a.change
			sound.Change = &v
		}
		s.Sound = sound
	}
	return s
}

// Busy reports whether a job is running: a connection being retried, or one
// being dropped.
func (a *App) Busy() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.job != nil && !a.job.view.Done
}

// Connect starts retrying the connection of the device with that address. It
// returns at once; Snapshot shows the progress.
func (a *App) Connect(address string) error {
	j, ctx, addr, err := a.begin(address, KindConnect, PhaseStarting)
	if err != nil {
		return err
	}
	go a.run(ctx, j, addr)
	return nil
}

// Disconnect drops the connection to the device with that address, and waits for
// Windows to report it. It returns at once; Snapshot shows the progress. The
// device stays paired, so it can be connected again.
func (a *App) Disconnect(address string) error {
	j, ctx, addr, err := a.begin(address, KindDisconnect, PhaseDisconnecting)
	if err != nil {
		return err
	}
	go a.runDisconnect(ctx, j, addr)
	return nil
}

// begin checks a request for a job and creates the job. Only one job runs at a
// time, and only for a device in the list. The caller starts the goroutine that
// runs the job, which must call a.wg.Done when it ends.
func (a *App) begin(address, kind, phase string) (*job, context.Context, bt.Address, error) {
	addr, err := bt.ParseAddress(address)
	if err != nil {
		return nil, nil, 0, err
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closing {
		return nil, nil, 0, ErrClosing
	}
	if a.job != nil && !a.job.view.Done {
		return nil, nil, 0, ErrBusy
	}
	var dev *DeviceView
	for i := range a.devices {
		if a.devices[i].Address == addr.String() {
			dev = &a.devices[i]
			break
		}
	}
	if dev == nil {
		return nil, nil, 0, ErrUnknownDevice
	}

	ctx, cancel := context.WithCancel(a.ctx)
	j := &job{
		cancel: cancel,
		view: JobView{
			Kind:      kind,
			Address:   dev.Address,
			Name:      dev.Name,
			Phase:     phase,
			StartedAt: a.now().UnixMilli(),
		},
	}
	a.job = j
	a.wg.Add(1)
	return j, ctx, addr, nil
}

// SwitchOutput makes the output with that ID the sound output of Windows, as
// audio.Switch does it. It returns at once; Snapshot shows the outcome. It does
// not wait for a connection job, and is not one.
func (a *App) SwitchOutput(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closing {
		return ErrClosing
	}
	if a.change != nil && !a.change.Done {
		return ErrSwitching
	}
	i := slices.IndexFunc(a.outputs, func(o OutputView) bool { return o.ID == id })
	if i < 0 {
		return ErrUnknownOutput
	}
	change := &ChangeView{ID: id, Name: a.outputs[i].Name}
	a.change = change
	a.wg.Add(1)
	go a.runSwitch(change)
	return nil
}

// runSwitch changes the output and records how it went.
func (a *App) runSwitch(change *ChangeView) {
	defer a.wg.Done()
	_, err := audio.Switch(a.cfg.Audio, change.ID, a.log)
	a.refreshSound() // before the change is reported as done, so the list is right by then

	a.mu.Lock()
	defer a.mu.Unlock()
	change.Done = true
	change.DoneAt = a.now().UnixMilli()
	if err != nil {
		change.Error = err.Error()
		change.ErrorCode = outputErrorCode(err)
	}
}

func outputErrorCode(err error) string {
	switch {
	case errors.Is(err, audio.ErrNotFound):
		return "output-gone"
	case errors.Is(err, audio.ErrNotSwitched):
		return "not-switched"
	case errors.Is(err, audio.ErrUnsupported):
		return "unsupported"
	}
	return ""
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
// devices and outputs, and waits for a request that Windows is working on to
// return, because stopping in the middle of it could leave a service switched
// off. A change of the output is waited for too. It reports false when that took
// longer than timeout. It may be called more than once.
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
		a.refreshSound()
	}
}

// refreshSound reads the sound outputs and which one is the default.
func (a *App) refreshSound() {
	if a.cfg.Audio == nil {
		return
	}
	a.soundMu.Lock()
	defer a.soundMu.Unlock()
	state, err := a.cfg.Audio.State()

	outputs := []OutputView{}
	text := ""
	if err != nil {
		text = err.Error()
	} else {
		for _, o := range state.Outputs {
			outputs = append(outputs, OutputView{
				ID:      o.ID,
				Name:    o.Name,
				Kind:    string(o.Kind),
				Current: o.ID == state.Defaults[audio.RoleConsole],
			})
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.outputs = outputs
	if text != a.soundError { // log a change, not every read
		a.soundError = text
		if text != "" {
			a.log.Warn("the sound outputs cannot be read", "problem", text)
		} else {
			a.log.Info("the sound outputs can be read again")
		}
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

// runDisconnect drops one connection and records how it went.
func (a *App) runDisconnect(ctx context.Context, j *job, addr bt.Address) {
	defer a.wg.Done()
	engine := latch.New(a.cfg.Adapter, a.cfg.Policy, a.log)
	err := engine.Disconnect(ctx, addr)

	a.mu.Lock()
	v := &j.view
	v.Done = true
	v.DoneAt = a.now().UnixMilli()
	v.Elapsed = v.DoneAt - v.StartedAt
	switch {
	case err == nil, errors.Is(err, bt.ErrNotConnected): // either way, it is disconnected
		v.Phase = PhaseDisconnected
	case errors.Is(err, context.Canceled):
		v.Phase = PhaseCancelled
	default:
		v.Phase = PhaseFailed
		v.Error = err.Error()
		v.ErrorCode = errorCode(err)
	}
	a.mu.Unlock()
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
	case errors.Is(err, latch.ErrStillConnected):
		return "still-connected"
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
