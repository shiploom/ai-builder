const card: React.CSSProperties = {
  border: '1px solid var(--fd-border)',
  borderRadius: '12px',
  padding: '20px',
  background: 'var(--fd-card)',
};

const muted: React.CSSProperties = { color: 'var(--fd-muted-foreground)' };

export default function HomePage() {
  return (
    <main style={{ maxWidth: '880px', margin: '0 auto', padding: '64px 24px' }}>
      <p style={{ ...muted, margin: 0 }}>Shiploom Core · MIT · offline-first</p>
      <h1 style={{ fontSize: '44px', lineHeight: 1.1, margin: '12px 0' }}>
        The thinnest portable layer that closes the AI verification gap
      </h1>
      <p style={{ fontSize: '18px', ...muted }}>
        Specifier → Implementer → Verifier, coordinated by a deterministic
        orchestrator plus human gates. Done is computed from locked acceptance
        criteria, a hidden oracle, independent verification, and deterministic
        gates — never claimed by the builder.
      </p>
      <pre
        style={{
          background: 'var(--fd-card)',
          border: '1px solid var(--fd-border)',
          borderRadius: '8px',
          padding: '12px 16px',
          overflowX: 'auto',
        }}
      >
        pipx install git+https://github.com/shiploom/ai-builder.git
      </pre>
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))',
          gap: '16px',
          marginTop: '32px',
        }}
      >
        <div style={card}>
          <h3 style={{ marginTop: 0 }}>Portable core</h3>
          <p style={muted}>
            Markdown-first artifacts, base-spec skills, and MCP capability
            references that run on Claude Code, OpenCode, and any AGENTS.md
            harness — no lock-in.
          </p>
        </div>
        <div style={card}>
          <h3 style={{ marginTop: 0 }}>Independent verification</h3>
          <p style={muted}>
            Fresh-context verifier, hidden oracle vault, deterministic gates,
            hash-locked acceptance. Same-model self-check is blocked by design.
          </p>
        </div>
        <div style={card}>
          <h3 style={{ marginTop: 0 }}>Human control</h3>
          <p style={muted}>
            Default-deny on infra, spend, destructive, and prod actions.
            Deny-means-deny, hash-chained audit, plain-language approvals.
          </p>
        </div>
      </div>
      <h2 style={{ marginTop: '48px' }}>Start here</h2>
      <ul>
        <li>
          <a href="/docs">Consumer manual (J1–J4)</a> — idea to MVP, brownfield
          fixes, team features, hardening
        </li>
        <li>
          <a href="/docs/install">Install</a> — pipx, uvx, source, verify
        </li>
        <li>
          <a href="/docs/policy-cookbook">Policy cookbook</a> — default-deny
          packs, gates, approvals
        </li>
      </ul>
      <p style={muted}>
        Core free under MIT. You bring the harness and the model keys; Shiploom
        brings the method, the contracts, and the gates.
      </p>
    </main>
  );
}
