<script lang="ts">
  const loginHref = "/auth/google/start";
</script>

<div class="login-screen pattern">
  <div class="login-card">
    <span class="login-kicker">Tinker workspace</span>
    <h1>Tinker</h1>
    <p>Sign in with your Google account to continue.</p>

    <a class="login-button interact-bounce" href={loginHref}>Sign in with Google</a>
  </div>
</div>

<style>
  .login-screen {
    flex: 1;
    min-height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: var(--space-8);
  }

  .login-card {
    width: min(420px, 90%);
    background: var(--color-surface-base);
    border: 1px solid var(--color-border-subtle);
    border-radius: var(--radius-xl);
    padding: var(--space-8);
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-3);
    text-align: center;
    box-shadow: var(--shadow-card);
  }

  .login-kicker {
    font-size: 0.7rem;
    font-weight: 700;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--color-text-secondary);
  }

  h1 {
    margin: 0;
    font-family: var(--font-family-display);
    font-size: clamp(3rem, 12vw, 4.8rem);
    font-weight: 500;
    letter-spacing: -0.06em;
    line-height: 0.9;
    color: var(--color-text-heading);
  }

  p {
    margin: 0;
    color: var(--color-text-secondary);
    font-size: 0.92rem;
    text-align: center;
    line-height: 1.6;
  }

  .login-button {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-height: 44px;
    padding: 0 var(--space-5);
    margin-top: var(--space-3);
    border: 1px solid var(--color-border-subtle);
    border-radius: var(--radius-md);
    color: var(--color-text-secondary);
    background: var(--color-surface-base);
    font-size: 0.92rem;
    font-weight: 600;
    text-decoration: none;
    box-shadow: none;

    &:hover {
      color: var(--color-text-body);
      border-color: var(--color-border-accent);
    }
  }
</style>
