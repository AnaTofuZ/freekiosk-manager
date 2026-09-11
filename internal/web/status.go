package web

import (
	"fmt"
	"time"
)

type statusView struct {
	State       string      `json:"state"`
	StateClass  string      `json:"stateClass"`
	Error       string      `json:"error"`
	Stale       bool        `json:"stale"`
	LastSuccess string      `json:"lastSuccess"`
	LastAttempt string      `json:"lastAttempt"`
	Rows        []statusRow `json:"rows"`
}

type statusRow struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

func statusViewFor(s Snapshot, lang string) statusView {
	clock := func(t *time.Time) string {
		if t == nil {
			return translate(lang, "Never")
		}
		return t.Format(time.RFC3339)
	}
	view := statusView{State: "○ Waiting for status", LastSuccess: clock(s.LastSuccess), LastAttempt: clock(s.LastAttempt), Rows: []statusRow{}}
	if s.Online {
		view.State = "● Online"
		view.StateClass = "online"
	}
	if s.Error != nil {
		view.State = "● Error"
		view.StateClass = "offline"
		view.Error = s.Error.Message
		view.Stale = s.Status != nil
		switch s.Error.Kind {
		case "network", "timeout", "connection_refused":
			view.State = "● Offline"
		}
	}
	add := func(label string, p any, suffix string) {
		var value any
		switch v := p.(type) {
		case *int:
			if v == nil {
				return
			}
			value = *v
		case *string:
			if v == nil {
				return
			}
			value = *v
		case *bool:
			if v == nil {
				return
			}
			value = translate(lang, "No")
			if *v {
				value = translate(lang, "Yes")
			}
		}
		view.Rows = append(view.Rows, statusRow{Label: translate(lang, label), Value: fmt.Sprint(value) + suffix})
	}
	if d := s.Status; d != nil {
		if d.Webview != nil {
			add("Current URL", d.Webview.CurrentURL, "")
		}
		if d.Battery != nil {
			add("Battery", d.Battery.Level, " %")
			add("Charging", d.Battery.Charging, "")
		}
		if d.Screen != nil {
			add("Screen on", d.Screen.On, "")
			add("Screensaver", d.Screen.ScreensaverActive, "")
			add("Brightness", d.Screen.Brightness, " %")
		}
		if d.Audio != nil {
			add("Volume", d.Audio.Volume, " %")
		}
		if d.Wifi != nil {
			add("Wi-Fi connected", d.Wifi.Connected, "")
			add("Network", d.Wifi.SSID, "")
			add("Signal", d.Wifi.RSSI, " dBm")
			add("Wi-Fi IP", d.Wifi.IP, "")
		}
		if d.Device != nil {
			add("Device IP", d.Device.IP, "")
			add("Model", d.Device.Model, "")
		}
	}
	view.State = translate(lang, view.State)
	view.Error = translate(lang, view.Error)
	return view
}
