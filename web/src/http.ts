export async function request(url: string, init?: RequestInit): Promise<Response> {
  let response: Response;
  try {
    response = await fetch(url, { ...init, signal: AbortSignal.timeout(70000) });
  } catch (error) {
    throw new Error(
      error instanceof DOMException && error.name === "TimeoutError"
        ? "Manager request timed out"
        : "Request failed",
    );
  }
  if (!response.ok) {
    let message = `Request failed (HTTP ${response.status})`;
    if (response.headers.get("content-type")?.includes("application/json")) {
      const body: { message?: string } = await response.json();
      message = body.message ?? message;
    }
    throw new Error(message);
  }
  return response;
}
