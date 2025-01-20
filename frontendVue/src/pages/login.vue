<template>
  <div class="form-page">
    <h2>Login</h2>
    <form @submit.prevent="loginUser">
      <div class="form-group">
        <label for="username">Username</label>
        <input v-model="username" type="text" id="username" required />
      </div>
      <div class="form-group">
        <label for="password">Password</label>
        <input v-model="password" type="password" id="password" required />
      </div>
      <button class="auth-button" type="submit">Login</button>
    </form>
    <p v-if="message" class="message">{{ message }}</p>
  </div>
</template>

<script>
import axios from "axios";

export default {
  name: "LoginPage",
  data() {
    return {
      username: "",
      password: "",
      message: "",
    };
  },
    methods: {
    async loginUser() {
      try {
        const response = await axios.post("http://localhost:8080/login", {
          Username: this.username,
          Password: this.password,
        });
        this.message = response.data.message;

        // Redirect to the 2FA page with the username as a query parameter
        this.$router.push({ path: "/2fa", query: { username: this.username } });
      } catch (error) {
        this.message = error.response?.data?.error || "Login failed.";
      }
    },
  },
};
</script>
