<script lang="ts">
  const loginHref = "/auth/google/start";
</script>

<div class="login-screen">
  <div class="login-card">
    <h1>Tinker</h1>
    <p>Sign in with your Google account to continue.</p>

    <a class="login-button" href={loginHref}>Sign in with Google</a>
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
    border: 1px solid var(--color-border-accent);
    border-radius: var(--radius-xl);
    padding: var(--space-6);
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-4);
    font-family: var(--font-family-display);
  }

  h1 {
    margin: 0;
    font-size: 1.5rem;
  }

  p {
    margin: 0;
    color: var(--color-text-body);
    font-size: 0.92rem;
    text-align: center;
  }

  .login-button {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-height: 44px;
    padding: 0 var(--space-5);
    border: 1px solid var(--color-border-accent);
    border-radius: var(--radius-full);
    color: var(--color-text-heading);
    background: var(--color-surface-raised);
    font-size: 0.92rem;
    text-decoration: none;
  }
</style>
