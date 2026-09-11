import { DeviceCard } from "./DeviceCard";

export function Page(props: {
  text: Record<string, string>;
  title: string;
  detailID: string;
  cards: { id: string; name: string }[];
}) {
  return (
    <div>
      <header>
        <a className="brand" href="/">
          FreeKiosk <span>manager</span>
        </a>
        <nav aria-label="Language / 言語">
          <a href="?lang=en" lang="en">
            English
          </a>{" "}
          /{" "}
          <a href="?lang=ja" lang="ja">
            日本語
          </a>
        </nav>
        <span className="lan">{props.text.HOMENETWORK}</span>
      </header>
      <main>
        <div className="heading">
          <div>
            <p className="eyebrow">{props.text.YOURDISPLAYS}</p>
            <h1>{props.title}</h1>
          </div>
          <p className="hint">
            {props.text.StatusRefreshesEvery20Seconds}
            <br />
            {props.text.HiddenTabsPauseAutomatically}
          </p>
        </div>
        <noscript>
          <p className="error">
            <span lang="en">Enable JavaScript for live status and device controls.</span>
            <br />
            <span lang="ja">状態の更新とデバイス操作にはJavaScriptを有効にしてください。</span>
          </p>
        </noscript>
        {props.detailID !== "" && <a href="/">{props.text.AllDevices}</a>}
        <div className="cards">
          {props.cards.map((card) => (
            <DeviceCard
              key={card.id}
              id={card.id}
              name={card.name}
              text={props.text}
              controls={props.detailID === card.id}
            />
          ))}
        </div>
        <footer>{props.text.FreeKioskLANManagementScreenshotsAreNeverPolled}</footer>
      </main>
    </div>
  );
}
