"use client";

import { createEffect, createSignal, onCleanup } from "@barefootjs/client";
import { request } from "../http";
import { locale, translate } from "../i18n";

export function Controls(props: {
  text: Record<string, string>;
  deviceID: string;
  status?: {
    screen?: { on?: boolean; brightness?: number };
    audio?: { volume?: number };
    webview?: { currentUrl?: string };
  };
  onUpdated: () => void;
}) {
  const [busy, setBusy] = createSignal(false);
  const [message, setMessage] = createSignal(props.text.Ready);
  const [failed, setFailed] = createSignal(false);
  const [imageURL, setImageURL] = createSignal("");
  const [captured, setCaptured] = createSignal("");
  const [screenOn, setScreenOn] = createSignal(false);
  const [brightness, setBrightness] = createSignal(50);
  const [volume, setVolume] = createSignal(20);
  const [currentURL, setCurrentURL] = createSignal("");
  createEffect(() => {
    const status = props.status;
    if (typeof status?.screen?.on === "boolean") setScreenOn(status.screen.on);
    if (typeof status?.screen?.brightness === "number") setBrightness(status.screen.brightness);
    if (typeof status?.audio?.volume === "number") setVolume(status.audio.volume);
    if (typeof status?.webview?.currentUrl === "string") setCurrentURL(status.webview.currentUrl);
  });
  onCleanup(() => {
    if (imageURL()) URL.revokeObjectURL(imageURL());
  });

  const submit = async (event: Event) => {
    event.preventDefault();
    const form = event.target;
    if (!(form instanceof HTMLFormElement) || busy()) return;
    setBusy(true);
    setFailed(false);
    setMessage(props.text.Working);
    try {
      const body: Record<string, string | number> = {};
      for (const [key, value] of new FormData(form))
        body[key] = key === "value" ? Number(value) : String(value);
      if (form.dataset.action === "screen") body.state = screenOn() ? "on" : "off";
      const response = await request(`/api/devices/${props.deviceID}/${form.dataset.action}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      const data: { message: string } = await response.json();
      setMessage(translate(data.message));
      window.setTimeout(props.onUpdated, 1000);
    } catch (error) {
      setFailed(true);
      setMessage(error instanceof Error ? translate(error.message) : props.text.RequestFailed);
    } finally {
      setBusy(false);
    }
  };
  const screenshot = async () => {
    if (busy()) return;
    setBusy(true);
    setFailed(false);
    setMessage(props.text.Capturing);
    try {
      const response = await request(`/api/devices/${props.deviceID}/screenshot`);
      const next = URL.createObjectURL(await response.blob());
      if (imageURL()) URL.revokeObjectURL(imageURL());
      setImageURL(next);
      setCaptured(new Date().toLocaleString(locale));
      setMessage(props.text.ScreenshotRefreshed);
    } catch (error) {
      setFailed(true);
      setMessage(error instanceof Error ? translate(error.message) : props.text.ScreenshotFailed);
    } finally {
      setBusy(false);
    }
  };
  return (
    <section
      className="controls"
      aria-label={props.text.DeviceControls}
      aria-busy={busy()}
      onSubmit={submit}
    >
      <div className="actions">
        <form data-action="reload">
          <button disabled={busy()}>{props.text.Reload}</button>
        </form>
        <form data-action="clear-cache">
          <button className="secondary" disabled={busy()}>
            {props.text.ClearCache}
          </button>
        </form>
        <form data-action="screen">
          <label className="switch">
            <input
              type="checkbox"
              role="switch"
              checked={screenOn()}
              disabled={busy()}
              onChange={(event) => {
                const input = event.currentTarget as HTMLInputElement;
                setScreenOn(input.checked);
                input.form?.requestSubmit();
              }}
            />
            <span>{props.text.ScreenPower}</span>
          </label>
        </form>
      </div>
      <div className="settings">
        <form data-action="brightness">
          <label for="brightness">{props.text.BrightnessLabel}</label>
          <div className="input-row">
            <input
              type="range"
              min="0"
              max="100"
              step="1"
              value={brightness()}
              aria-label={props.text.BrightnessLabel}
              onInput={(event) =>
                setBrightness((event.currentTarget as HTMLInputElement).valueAsNumber)
              }
            />
            <input
              id="brightness"
              name="value"
              type="number"
              min="0"
              max="100"
              step="1"
              value={brightness()}
              onInput={(event) => {
                const value = (event.currentTarget as HTMLInputElement).valueAsNumber;
                if (!Number.isNaN(value)) setBrightness(value);
              }}
              required
            />
            <button disabled={busy()}>{props.text.SetBrightness}</button>
          </div>
        </form>
        <form data-action="volume">
          <label for="volume">{props.text.VolumeLabel}</label>
          <div className="input-row">
            <input
              type="range"
              min="0"
              max="100"
              step="1"
              value={volume()}
              aria-label={props.text.VolumeLabel}
              onInput={(event) =>
                setVolume((event.currentTarget as HTMLInputElement).valueAsNumber)
              }
            />
            <input
              id="volume"
              name="value"
              type="number"
              min="0"
              max="100"
              step="1"
              value={volume()}
              onInput={(event) => {
                const value = (event.currentTarget as HTMLInputElement).valueAsNumber;
                if (!Number.isNaN(value)) setVolume(value);
              }}
              required
            />
            <button disabled={busy()}>{props.text.SetVolume}</button>
          </div>
        </form>
        <form data-action="url" className="wide">
          <label for="url">{props.text.DisplayURL}</label>
          <div className="input-row">
            <input
              id="url"
              name="url"
              type="url"
              value={currentURL()}
              placeholder="https://signage.home/"
              maxlength={8192}
              required
            />
            <button disabled={busy()}>{props.text.UpdateURL}</button>
          </div>
        </form>
        <form data-action="toast">
          <label for="toast">{props.text.ToastMessage}</label>
          <input id="toast" name="text" maxlength={4096} placeholder={props.text.Hello} required />
          <button disabled={busy()}>{props.text.SendToast}</button>
        </form>
        <form data-action="tts">
          <label for="tts">{props.text.TextToSpeech}</label>
          <input
            id="tts"
            name="text"
            maxlength={4096}
            placeholder={props.text.DinnerIsReady}
            required
          />
          <label for="language">{props.text.LanguageOptional}</label>
          <input id="language" name="language" maxlength={35} placeholder="ja" />
          <button disabled={busy()}>{props.text.Speak}</button>
        </form>
      </div>
      <p className={failed() ? "message error" : "message"} role="status" aria-live="polite">
        {message() || props.text.Ready}
      </p>
      <div className="screenshot">
        <div className="card-title">
          <h2>{props.text.Screenshot}</h2>
          <button type="button" onClick={screenshot} disabled={busy()}>
            {props.text.RefreshScreenshot}
          </button>
        </div>
        <p className="hint">
          {props.text.CapturedOnlyWhenRequestedTheLastImageStaysVisibleIfCaptureFails}
        </p>
        {imageURL() !== "" && <img src={imageURL()} alt={props.text.LastCapturedTabletScreen} />}
        <p className="hint">
          {captured() !== ""
            ? `${props.text.Captured} ${captured()}`
            : props.text.NoScreenshotCapturedYet}
        </p>
      </div>
    </section>
  );
}
