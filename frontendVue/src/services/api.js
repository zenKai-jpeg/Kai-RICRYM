import axios from 'axios';

// Base URL for the API (adjust if needed)
const API_BASE_URL = 'http://localhost:8080'; // Make sure to update this to your backend URL

// Create an Axios instance with default configurations
const api = axios.create({
  baseURL: API_BASE_URL,
  timeout: 5000, // Set a timeout for requests (e.g., 5000ms)
});

// Function to get player accounts with pagination, sorting, and search support
export const getAccounts = async (page = 1, limit = 10, search = '', classFilter = '', minScore = null, maxScore = null, sort = 'rank', order = 'asc') => {
  try {
    const response = await api.get('/accounts', {
      params: {
        page,
        limit,
        search,
        class: classFilter, // Use the correct query parameter name for class
        minScore,        // Use the correct query parameter name for minScore
        maxScore,        // Use the correct query parameter name for maxScore
        sort,
        order,
      },
    });

    // Check if response status is 200 (OK)
    if (response.status === 200) {
      return response.data; // Assuming the response includes `data`, `total`, and `totalPages`
    } else {
      console.error('Unexpected response status:', response.status);
      throw new Error('Unexpected response status');
    }
  } catch (error) {
    // Check for network-related errors or request timeouts
    if (error.code === 'ECONNABORTED') {
      console.error('Request timeout:', error);
    } else if (error.response) {
      // The request was made and the server responded with an error
      console.error('Server responded with error:', error.response.data);
    } else {
      // Something else went wrong
      console.error('Error fetching accounts:', error.message);
    }
    throw error; // Re-throw the error after logging it
  }
};

// You can add more functions to interact with other API endpoints if needed
