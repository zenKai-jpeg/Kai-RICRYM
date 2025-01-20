<template>
    <div class="verify-email-page">
      <h2>Check Your Email</h2>
      <p>Please check your inbox to verify your account.</p>
      <button @click="goHome" class="button">Go to Homepage</button>
    </div>
  </template>
  
  <script>
  export default {
    name: "VerifyEmailPage",
    methods: {
      goHome() {
        this.$router.push("/");
      },
    },
  };
  </script>