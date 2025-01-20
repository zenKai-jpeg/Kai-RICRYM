<script setup>
import { useRouter } from 'vue-router';
import '@/assets/main.css';

const router = useRouter();
</script>

<template>
  <div>
    <!-- Header Section -->
    <header class="header">
      <div class="header-logo-container">
        <router-link to="/">
          <img
            src="https://www.wira.org/assets/wira-logo-full.cc14df8b.png"
            alt="Wira Logo"
            class="header-logo"
          />
        </router-link>
      </div>
      <div class="button-container">
        <button class="button" @click="router.push('/register')">Register</button>
        <button class="button" @click="router.push('/login')">Login</button>
      </div>
    </header>

    <!-- Main Content Section -->
    <main>
      <router-view /> <!-- Dynamic component rendering -->
    </main>
  </div>
</template>
