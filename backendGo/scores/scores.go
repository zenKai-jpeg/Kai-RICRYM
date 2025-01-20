package scores

import (
	"database/sql"
	"log"

	"github.com/brianvoe/gofakeit"
)

// GenerateScoresForLoggedInUser generates scores for the user who logged in successfully.
func GenerateScoresForLoggedInUser(db *sql.DB, accID uint64) {
	for classID := 1; classID <= 8; classID++ {
		var charID uint64
		err := db.QueryRow("INSERT INTO characters (acc_id, class_id) VALUES ($1, $2) RETURNING char_id", accID, classID).Scan(&charID)
		if err != nil {
			log.Printf("Error creating character for account ID %d and class ID %d: %v", accID, classID, err)
			continue
		}

		rewardScore := gofakeit.Number(10, 1000)

		_, err = db.Exec("INSERT INTO scores (char_id, reward_score) VALUES ($1, $2)", charID, rewardScore)
		if err != nil {
			log.Printf("Error creating score for account ID %d, character ID %d, class ID %d: %v", accID, charID, classID, err)
			continue
		}
		log.Printf("Score created for character ID %d, class ID %d, score: %d", charID, classID, rewardScore)
	}

	log.Printf("Scores generated successfully for account ID %d.", accID)
}
