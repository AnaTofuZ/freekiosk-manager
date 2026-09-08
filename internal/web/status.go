package web

import (
	"fmt"
	"time"

	"github.com/AnaTofuZ/freekiosk-manager/web/views"
)

func statusProps(s Snapshot, lang string) views.StatusProps {
	clock := func(t *time.Time) string {
		if t == nil {
			return translate(lang, "Never")
		}
		return t.Format(time.RFC3339)
	}
	input := views.StatusInput{Text: translations(lang), State: "○ Waiting for status", LastSuccess: clock(s.LastSuccess), LastAttempt: clock(s.LastAttempt), Rows: []views.StatusRowsItem{}}
	if s.Online {
		input.State = "● Online"
		input.StateClass = "online"
	}
	if s.Error != nil {
		input.State = "● Error"
		input.StateClass = "offline"
		input.Error = s.Error.Message
		input.Stale = s.Status != nil
		switch s.Error.Kind {
		case "network", "timeout", "connection_refused":
			input.State = "● Offline"
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
		input.Rows = append(input.Rows, views.StatusRowsItem{Label: translate(lang, label), Value: fmt.Sprint(value) + suffix})
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
	input.State = translate(lang, input.State)
	input.Error = translate(lang, input.Error)
	return views.NewStatusProps(input)
}
