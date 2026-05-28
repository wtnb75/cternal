<template>
  <div class="welcome">
    <div class="welcome-inner">
      <div class="icon">⬡</div>
      <p class="hint">Select a container from the sidebar to start a session.</p>
    </div>
  </div>
</template>

<style scoped>
.welcome {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  background: #1e1e2e;
}

.welcome-inner {
  text-align: center;
  color: #45475a;
}

.icon {
  font-size: 3rem;
  margin-bottom: 1rem;
  line-height: 1;
}

.hint {
  font-size: 0.9rem;
}
</style>
