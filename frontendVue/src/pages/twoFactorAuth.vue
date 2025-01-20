<template>
  <div class="twofa-page">
    <h2>Enter 2FA Code</h2>
    <form @submit.prevent="verify2FA">
      <div>
        <label for="twofaCode">2FA Code</label>
        <input v-model="twofaCode" type="text" id="twofaCode" required />
      </div>
      <button type="submit">Verify Code</button>
    </form>
    <p v-if="message">{{ message }}</p>
  </div>
</template>

<script>
import axios from 'axios';

export default {
  name: 'TwoFactorAuth',
  data() {
    return {
      twofaCode: '',
      message: ''
    };
  },
  methods: {
    async verify2FA() {
      try {
        const response = await axios.post("http://localhost:8080/verify-2fa", {
          Username: this.$route.query.username, // Get username from the query string
          TwoFACode: this.twofaCode, // User's input
        });
        this.message = response.data.message;
        this.$router.push("/"); // Redirect on success
      } catch (error) {
        this.message = error.response?.data?.error || "Error verifying 2FA.";
      }
    },
  },
};
</script>
