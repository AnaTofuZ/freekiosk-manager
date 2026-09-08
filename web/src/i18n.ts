import japanese from "../locales/ja.json";
import english from "../locales/en.json";

export const locale = document.documentElement.lang;

export function translate(message: string): string {
  if (locale !== "ja") return message;
  const text: Record<string, string> = japanese;
  const key = Object.entries(english).find(([, value]) => value === message)?.[0];
  if (key) return text[key];
  const http = /^(?:Device returned HTTP |Request failed \(HTTP )(\d{3})\)?$/.exec(message);
  if (http) return `HTTPエラー (${http[1]})`;
  return message;
}
