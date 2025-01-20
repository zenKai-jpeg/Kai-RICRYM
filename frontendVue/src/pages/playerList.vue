<template>
    <div class="logo-container">
        <img
          src="https://www.wira.org/assets/wira-logo-full.cc14df8b.png"
          alt="Wira Logo"
          class="logo"
        />
    </div>
    
    <div class="player-list-container">
      <!-- Search Filters -->
      <div class="filters-container">
        <!-- Username Search -->
        <input
          v-model.trim="searchTerm"
          placeholder="Search by username..."
          class="filter-input"
        />
  
        <!-- Class Dropdown -->
        <select v-model="selectedClass" class="filter-select">
          <option value="">Select Class</option>
          <option v-for="i in 8" :key="i" :value="i">
            {{ getClassName(i) }}
          </option>
        </select>
  
        <!-- Score Range Filters -->
        <input
          type="number"
          v-model.number="minScore"
          placeholder="Min Score"
          class="filter-input"
        />
        <input
          type="number"
          v-model.number="maxScore"
          placeholder="Max Score"
          class="filter-input"
        />
  
        <!-- Search Button -->
        <button @click="triggerSearch" class="button">Search</button>
      </div>
  
    <div class="separator">
    <img src="@/assets/separator.png" alt="Separator">
    </div>

      <!-- Loading indicator -->
      <div v-if="loading" class="loading">Loading...</div>
  
      <!-- Table -->
      <table v-else class="player-table">
        <thead>
          <tr>
            <th @click="sortTable('rank')">
              RANK
              <span v-if="sortBy === 'rank'">{{ sortOrder === 'asc' ? '↑' : '↓' }}</span>
            </th>
            <th @click="sortTable('username')">
              USERNAME
              <span v-if="sortBy === 'username'">{{ sortOrder === 'asc' ? '↑' : '↓' }}</span>
            </th>
            <th @click="sortTable('class_id')">
              CLASS
              <span v-if="sortBy === 'class_id'">{{ sortOrder === 'asc' ? '↑' : '↓' }}</span>
            </th>
            <th @click="sortTable('score')">
              SCORE
              <span v-if="sortBy === 'score'">{{ sortOrder === 'asc' ? '↑' : '↓' }}</span>
            </th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="player in players" :key="player.AccID">
            <td>{{ player.Rank }}</td>
            <td>{{ player.Username }}</td>
            <td>{{ getClassName(player.ClassID) }}</td>
            <td>{{ player.Score }}</td>
          </tr>
        </tbody>
      </table>
      
      <!-- Pagination controls -->
      <div class="pagination-controls">
        <button
          @click="previousPage"
          :disabled="currentPage === 1"
          class="pagination-button"
        >
          ◄ Previous
        </button>
        <span class="page-info">{{ currentPage }} / {{ totalPages }}</span>
        <button
          @click="nextPage"
          :disabled="currentPage === totalPages"
          class="pagination-button"
        >
          Next ►
        </button>
      </div>
    </div>
  </template>
  
  <script>
  import { getAccounts } from "@/services/api";
  
  export default {
    data() {
      return {
        players: [],
        currentPage: 1,
        itemsPerPage: 10,
        totalItems: 0,
        totalPages: 0,
        loading: false,
        sortBy: "rank",
        sortOrder: "asc",
        searchTerm: "",
        selectedClass: "",
        minScore: null,
        maxScore: null,
      };
    },
  
    methods: {
      async fetchPlayers() {
        this.loading = true;
        try {
          const response = await getAccounts(
            this.currentPage,
            this.itemsPerPage,
            this.searchTerm,
            this.selectedClass,
            this.minScore,
            this.maxScore,
            this.sortBy,
            this.sortOrder
          );
          console.log(response);
          this.players = response.data;
          this.totalItems = response.total;
          this.totalPages = response.totalPages;
        } catch (error) {
          console.error("Error fetching players:", error);
        } finally {
          this.loading = false;
        }
      },
  
      // This method is now only for updating the searchTerm
      handleSearch() {
        // No immediate action, will be triggered by the Search button
      },
  
      // This method is now only for updating the filter values
      handleFilterChange() {
        // No immediate action, will be triggered by the Search button
      },
  
      // Method to trigger the search with current filter values
      triggerSearch() {
        this.currentPage = 1; // Reset to first page on new search
        this.fetchPlayers();
      },
  
      sortTable(column) {
        if (this.sortBy === column) {
          this.sortOrder = this.sortOrder === "asc" ? "desc" : "asc";
        } else {
          this.sortBy = column;
          this.sortOrder = "asc";
        }
        this.fetchPlayers();
      },
  
      previousPage() {
        if (this.currentPage > 1) {
          this.currentPage--;
          this.fetchPlayers();
        }
      },
  
      nextPage() {
        if (this.currentPage < this.totalPages) {
          this.currentPage++;
          this.fetchPlayers();
        }
      },
  
      // New method to return class names based on the class ID
      getClassName(classId) {
        switch (classId) {
          case 1:
            return "Keris Warrior";
          case 2:
            return "Wayang Puppeteer";
          case 3:
            return "Penjaga Hutan";
          case 4:
            return "Silat Master";
          case 5:
            return "Naga Berserker";
          case 6:
            return "Orang Laut Raider";
          case 7:
            return "Bomoh Shaman";
          case 8:
            return "Empu Blacksmith";
          default:
            return "Unknown Class";
        }
      },
    },
  
    watch: {
      // Optional: If you still want some immediate feedback, like resetting page on search term change
      searchTerm() {
        // You can add logic here if needed, for example, reset page number
        // if the user starts typing a new search term.
        // However, the actual data fetching is still triggered by the Search button.
      },
    },
  
    mounted() {
      // Initial fetch when the component is mounted (without any filters)
      this.fetchPlayers();
    },
  };
  </script>