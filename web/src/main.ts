import { createEffect, createSignal } from "@barefootjs/client";
import { request } from "./http";
import { locale, translate } from "./i18n";

const refreshers = new Map<string, () => Promise<void>>();
for (const card of document.querySelectorAll<HTMLElement>("[data-device]")) {
  const id = card.dataset.device!;
  const button = card.querySelector<HTMLButtonElement>("[data-refresh]")!;
  const status = card.querySelector<HTMLElement>("[data-status]")!;
  const [busy, setBusy] = createSignal(false);
  createEffect(() => {
    button.disabled = busy();
  });
  const refresh = async () => {
    if (busy()) return;
    setBusy(true);
    try {
      const response = await request(`/fragments/devices/${id}/status?lang=${locale}`);
      // The fragment is a compiled BarefootJS template, escaped by Go on the server.
      status.innerHTML = await response.text();
      card.querySelector("[data-poll-error]")?.remove();
    } catch {
      if (!card.querySelector("[data-poll-error]")) {
        const error = document.createElement("p");
        error.dataset.pollError = "";
        error.className = "error";
        error.textContent = translate("Manager connection failed. Displayed status is stale.");
        status.after(error);
      }
    } finally {
      setBusy(false);
    }
  };
  refreshers.set(id, refresh);
  button.addEventListener("click", () => void refresh());
  const poll = async () => {
    if (!document.hidden) await refresh();
    window.setTimeout(() => void poll(), 20000);
  };
  void poll();
}
document.addEventListener("device-updated", (event) => {
  if (event instanceof CustomEvent && typeof event.detail === "string")
    void refreshers.get(event.detail)?.();
});
