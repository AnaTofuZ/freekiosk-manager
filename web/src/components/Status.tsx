export function Status(props: {
  text: Record<string, string>;
  state: string;
  stateClass: string;
  error: string;
  stale: boolean;
  lastSuccess: string;
  lastAttempt: string;
  rows: { label: string; value: string }[];
}) {
  return (
    <div>
      <p className={`state ${props.stateClass}`}>{props.state}</p>
      {props.error !== "" && <p className="error">{props.error}</p>}
      {props.stale && <p className="hint">{props.text.ShowingTheLastSuccessfulStatus}</p>}
      <dl>
        {props.rows.map((row) => (
          <div key={row.label} className="status-row">
            <dt>{row.label}</dt>
            <dd>{row.value}</dd>
          </div>
        ))}
      </dl>
      <p className="timestamp">
        {props.text.LastSuccess} <time>{props.lastSuccess}</time>
        <br />
        {props.text.LastAttempt} <time>{props.lastAttempt}</time>
      </p>
    </div>
  );
}
