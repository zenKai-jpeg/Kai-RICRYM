import { createRouter, createWebHistory } from 'vue-router';
import LoginPage from './pages/Login.vue';
import RegisterPage from './pages/Register.vue';
import TwoFactorAuth from './pages/twoFactorAuth.vue';
import playerList from './pages/playerList.vue'; // Import the player list component

const routes = [
  { path: '/', name: 'Dashboard', component: playerList }, // Default route
  { path: '/register', name: 'Register', component: RegisterPage },
  { path: '/login', name: 'Login', component: LoginPage },
  { path: '/2fa', name: 'TwoFactorAuth', component: TwoFactorAuth }, // Add the 2FA route
];

const router = createRouter({
  history: createWebHistory(),
  routes,
});

export default router;
