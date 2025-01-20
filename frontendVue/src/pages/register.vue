<template>
  <div class="form-page">
    <h2>Register</h2>
    <form @submit.prevent="registerUser">
      <div class="form-group">
        <label for="username">Username</label>
        <input v-model="username" type="text" id="username" required />
      </div>
      <div class="form-group">
        <label for="email">Email</label>
        <input v-model="email" type="email" id="email" required />
      </div>
      <div class="form-group">
        <label for="password">Password</label>
        <input v-model="password" type="password" id="password" required />
      </div>
      <button class="auth-button" type="submit">Register</button>
    </form>
    <p v-if="message" class="message">{{ message }}</p>
  </div>
</template>

<script>
import axios from "axios";

export default {
  name: "RegisterPage",
  data() {
    return {
      username: "",
      email: "",
      password: "",
      message: "",
    };
  },
  methods: {
    async registerUser() {
      try {
        const response = await axios.post("http://localhost:8080/register", {
          Username: this.username,
          Email: this.email,
          Password: this.password,
        });
        this.message = response.data.message;
        this.$router.push("/verify-email");
      } catch (error) {
        this.message = error.response?.data?.error || "Registration failed.";
      }
    },
  },
};
</script>
