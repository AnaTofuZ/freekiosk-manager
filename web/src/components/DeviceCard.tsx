"use client";

import { createSignal, onCleanup, onMount } from "@barefootjs/client";
import { request } from "../http";
import { locale, translate } from "../i18n";
import { Controls } from "./Controls";

type DeviceStatus = {
  screen?: { on?: boolean; brightness?: number };
  audio?: { volume?: number };
  webview?: { currentUrl?: string };
};

type StatusView = {
  state: string;
  stateClass: string;
  error: string;
  stale: boolean;
  lastSuccess: string;
  lastAttempt: string;
  rows: { label: string; value: string }[];
};

export function DeviceCard(props: {
  id: string;
  name: string;
  text: Record<string, string>;
  controls: boolean;
}) {
  const [busy, setBusy] = createSignal(false);
  const [pollError, setPollError] = createSignal("");
  const [status, setStatus] = createSignal<DeviceStatus | undefined>(undefined);
  const [state, setState] = createSignal(props.text.WaitingForStatus);
  const [stateClass, setStateClass] = createSignal("");
  const [error, setError] = createSignal("");
  const [stale, setStale] = createSignal(false);
  const [lastSuccess, setLastSuccess] = createSignal(props.text.Never);
  const [lastAttempt, setLastAttempt] = createSignal(props.text.Never);
  const [rows, setRows] = createSignal<StatusView["rows"]>([]);
  let timer: number | undefined;

  const refresh = async () => {
    if (busy()) return;
    setBusy(true);
    try {
      const response = await request(`/api/devices/${props.id}/status?lang=${locale}`);
      const next: { status?: DeviceStatus; view: StatusView } = await response.json();
      setStatus(next.status);
      setState(next.view.state);
      setStateClass(next.view.stateClass);
      setError(next.view.error);
      setStale(next.view.stale);
      setLastSuccess(next.view.lastSuccess);
      setLastAttempt(next.view.lastAttempt);
      setRows(next.view.rows);
      setPollError("");
    } catch {
      setPollError(translate("Manager connection failed. Displayed status is stale."));
    } finally {
      setBusy(false);
    }
  };
  const poll = async () => {
    if (!document.hidden) await refresh();
    timer = window.setTimeout(() => void poll(), 20000);
  };
  onMount(() => void poll());
  onCleanup(() => {
    if (timer !== undefined) window.clearTimeout(timer);
  });

  return (
    <div className="device">
      <article className="card">
        <div className="card-title">
          <h2>
            <a href={`/devices/${props.id}`}>{props.name}</a>
          </h2>
          <code>{props.id}</code>
        </div>
        <section aria-label={props.text.DeviceStatus}>
          <p className={`state ${stateClass()}`}>{state() || props.text.WaitingForStatus}</p>
          {error() !== "" && <p className="error">{error()}</p>}
          {stale() && <p className="hint">{props.text.ShowingTheLastSuccessfulStatus}</p>}
          <dl>
            {rows().map((row) => (
              <div key={row.label} className="status-row">
                <dt>{row.label}</dt>
                <dd>{row.value}</dd>
              </div>
            ))}
          </dl>
          <p className="timestamp">
            {props.text.LastSuccess} <time>{lastSuccess() || props.text.Never}</time>
            <br />
            {props.text.LastAttempt} <time>{lastAttempt() || props.text.Never}</time>
          </p>
        </section>
        {pollError() !== "" && <p className="error">{pollError()}</p>}
        <button type="button" onClick={refresh} disabled={busy()}>
          {props.text.StatusRefresh}
        </button>
      </article>
      {props.controls && (
        <Controls deviceID={props.id} text={props.text} status={status()} onUpdated={refresh} />
      )}
    </div>
  );
}
