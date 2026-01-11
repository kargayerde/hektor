package router

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"log/slog"

	"relaypanel/internal/adb"
	"relaypanel/internal/db"
	"relaypanel/internal/device"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type API struct {
	Devices *device.Manager
	ADB     *adb.Client
}

type SetLabelRequest struct {
	Label string `json:"label"`
}

type statusWriter struct {
	http.ResponseWriter
	status int
	size   int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.size += n
	return n, err
}

func slogHTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{ResponseWriter: w}
		start := time.Now()
		next.ServeHTTP(sw, r)
		slog.Info("http",
			slog.Group("http",
				"method", r.Method,
				"path", r.URL.Path,
				"status", sw.status,
				"size", sw.size,
				"req_id", middleware.GetReqID(r.Context()),
				"dur", time.Since(start),
			),
		)
	})
}

func (a *API) getRelayStatesHandler(w http.ResponseWriter, r *http.Request) {
	states := a.Devices.RelayStates()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(states)
}

func (a *API) getStatusHandler(w http.ResponseWriter, r *http.Request) {
	states := a.Devices.RelayStates()
	devs := []device.DeviceState{
		{Name: "relays", State: "disconnected"},
		{Name: "buzzer", State: "disconnected"},
	}
	if a.Devices.GetDevice("relays") != nil {
		devs[0].State = "connected"
	}
	if a.Devices.GetDevice("buzzer") != nil {
		devs[1].State = "connected"
	}
	resp := struct {
		DeviceStates []device.DeviceState `json:"devices"`
		RelayStates  []device.RelayState  `json:"relays"`
	}{
		DeviceStates: devs,
		RelayStates:  states,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (a *API) toggleRelayHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := a.Devices.ToggleRelay(id); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	a.getRelayStatesHandler(w, r)
}

func (a *API) doorBuzzHandler(w http.ResponseWriter, r *http.Request) {
	if err := a.Devices.BuzzDoor(); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (a *API) setRelayLabelHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if len(id) != 1 || id[0] < '1' || id[0] > '8' {
		http.Error(w, "invalid relay id", http.StatusBadRequest)
		return
	}
	idx := int(id[0] - '1')

	var req SetLabelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	a.Devices.UpdateLabel(idx, req.Label)
	if err := db.UpdateRelayLabel(r.Context(), int64(idx+1), req.Label); err != nil {
		http.Error(w, "failed to update label in database", http.StatusInternalServerError)
		return
	}
	slog.Info("relay label updated", "relay_index", idx+1, "label", req.Label)
	w.WriteHeader(http.StatusOK)
}

func (a *API) tvDo(w http.ResponseWriter, r *http.Request, fn func() error) {
	if a.ADB == nil {
		http.Error(w, "adb client not configured", http.StatusServiceUnavailable)
		return
	}
	if err := fn(); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (a *API) tvVolumeUpHandler(w http.ResponseWriter, r *http.Request) {
	a.tvDo(w, r, func() error { return a.ADB.VolumeUp(r.Context()) })
}
func (a *API) tvVolumeDownHandler(w http.ResponseWriter, r *http.Request) {
	a.tvDo(w, r, func() error { return a.ADB.VolumeDown(r.Context()) })
}
func (a *API) tvPowerHandler(w http.ResponseWriter, r *http.Request) {
	a.tvDo(w, r, func() error { return a.ADB.PowerToggle(r.Context()) })
}
func (a *API) tvHomeHandler(w http.ResponseWriter, r *http.Request) {
	a.tvDo(w, r, func() error { return a.ADB.Home(r.Context()) })
}
func (a *API) tvBackHandler(w http.ResponseWriter, r *http.Request) {
	a.tvDo(w, r, func() error { return a.ADB.Back(r.Context()) })
}
func (a *API) tvMicMuteHandler(w http.ResponseWriter, r *http.Request) {
	a.tvDo(w, r, func() error { return a.ADB.MicMute(r.Context()) })
}
func (a *API) tvMediaPlayPauseHandler(w http.ResponseWriter, r *http.Request) {
	a.tvDo(w, r, func() error { return a.ADB.MediaPlayPause(r.Context()) })
}
func (a *API) tvMediaNextHandler(w http.ResponseWriter, r *http.Request) {
	a.tvDo(w, r, func() error { return a.ADB.NextTrack(r.Context()) })
}
func (a *API) tvMediaPrevHandler(w http.ResponseWriter, r *http.Request) {
	a.tvDo(w, r, func() error { return a.ADB.PreviousTrack(r.Context()) })
}
func (a *API) tvMediaStopHandler(w http.ResponseWriter, r *http.Request) {
	a.tvDo(w, r, func() error { return a.ADB.MediaStop(r.Context()) })
}
func (a *API) tvDpadUpHandler(w http.ResponseWriter, r *http.Request) {
	a.tvDo(w, r, func() error { return a.ADB.DpadUp(r.Context()) })
}
func (a *API) tvDpadDownHandler(w http.ResponseWriter, r *http.Request) {
	a.tvDo(w, r, func() error { return a.ADB.DpadDown(r.Context()) })
}
func (a *API) tvDpadLeftHandler(w http.ResponseWriter, r *http.Request) {
	a.tvDo(w, r, func() error { return a.ADB.DpadLeft(r.Context()) })
}
func (a *API) tvDpadRightHandler(w http.ResponseWriter, r *http.Request) {
	a.tvDo(w, r, func() error { return a.ADB.DpadRight(r.Context()) })
}
func (a *API) tvDpadCenterHandler(w http.ResponseWriter, r *http.Request) {
	a.tvDo(w, r, func() error { return a.ADB.DpadCenter(r.Context()) })
}
func (a *API) tvMenuHandler(w http.ResponseWriter, r *http.Request) {
	a.tvDo(w, r, func() error { return a.ADB.Menu(r.Context()) })
}
func (a *API) tvSettingsHandler(w http.ResponseWriter, r *http.Request) {
	a.tvDo(w, r, func() error { return a.ADB.Settings(r.Context()) })
}
func (a *API) tvSpeakerMuteHandler(w http.ResponseWriter, r *http.Request) {
	a.tvDo(w, r, func() error { return a.ADB.SpeakerMute(r.Context()) })
}
func (a *API) tvInputSourceHandler(w http.ResponseWriter, r *http.Request) {
	a.tvDo(w, r, func() error { return a.ADB.InputSource(r.Context()) })
}
func (a *API) tvFavouriteHandler(w http.ResponseWriter, r *http.Request) {
	a.tvDo(w, r, func() error { return a.ADB.Favourite(r.Context()) })
}

func Router(a *API) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.StripSlashes)
	r.Use(middleware.RequestID)
	r.Use(slogHTTP)

	r.Get("/status", a.getStatusHandler)
	r.Get("/relay/{id}", a.toggleRelayHandler)
	r.Get("/relay/states", a.getRelayStatesHandler)
	r.Post("/relay/setLabel/{id}", a.setRelayLabelHandler)
	r.Get("/door/buzz", a.doorBuzzHandler)

	r.Get("/tv/volume_up", a.tvVolumeUpHandler)
	r.Get("/tv/volume_down", a.tvVolumeDownHandler)
	r.Get("/tv/power", a.tvPowerHandler)
	r.Get("/tv/home", a.tvHomeHandler)
	r.Get("/tv/back", a.tvBackHandler)
	r.Get("/tv/mic_mute", a.tvMicMuteHandler)
	r.Get("/tv/media_play_pause", a.tvMediaPlayPauseHandler)
	r.Get("/tv/media_next", a.tvMediaNextHandler)
	r.Get("/tv/media_prev", a.tvMediaPrevHandler)
	r.Get("/tv/media_stop", a.tvMediaStopHandler)

	r.Get("/tv/dpad_up", a.tvDpadUpHandler)
	r.Get("/tv/dpad_down", a.tvDpadDownHandler)
	r.Get("/tv/dpad_left", a.tvDpadLeftHandler)
	r.Get("/tv/dpad_right", a.tvDpadRightHandler)
	r.Get("/tv/dpad_center", a.tvDpadCenterHandler)
	r.Get("/tv/menu", a.tvMenuHandler)
	r.Get("/tv/settings", a.tvSettingsHandler)
	r.Get("/tv/speaker_mute", a.tvSpeakerMuteHandler)
	r.Get("/tv/input_source", a.tvInputSourceHandler)
	r.Get("/tv/favourite", a.tvFavouriteHandler)

	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	staticDir := filepath.Join(exeDir, "static")

	r.Handle("/", http.FileServer(http.Dir(staticDir)))
	return r
}

func FormatRelayStatus(states []device.RelayState) (joined string, onCount int) {
	ss := make([]string, 0, len(states))
	for i, st := range states {
		ri := i + 1
		onOff := "OFF"
		if st.State {
			onOff = "ON"
			onCount++
		}
		if st.Label != "" {
			ss = append(ss, "Relay "+strconv.Itoa(ri)+" ("+st.Label+") "+onOff)
		} else {
			ss = append(ss, "Relay "+strconv.Itoa(ri)+": "+onOff)
		}
	}
	return stringsJoin(ss, " | "), onCount
}

func stringsJoin(a []string, sep string) string {
	switch len(a) {
	case 0:
		return ""
	case 1:
		return a[0]
	}
	n := len(sep) * (len(a) - 1)
	for i := 0; i < len(a); i++ {
		n += len(a[i])
	}
	var b = make([]byte, 0, n)
	for i, s := range a {
		if i > 0 {
			b = append(b, sep...)
		}
		b = append(b, s...)
	}
	return string(b)
}
