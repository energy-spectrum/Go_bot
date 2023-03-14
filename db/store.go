package db

import (
	"database/sql"
	"log"
	"time"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) Store {
	return Store{
		db: db,
	}
}

func (s *Store) SaveRequest(userID int, cryptocurrency string) error {
	const saveRequestQuery = `
INSERT INTO requests (user_id, cryptocurrency)
VALUES ($1, $2)
`
	_, err := s.db.Exec(saveRequestQuery, userID, cryptocurrency)
	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func (s *Store) CountRequests(userID int) (int, error) {
	const countRequestsQuery = `
SELECT COUNT(*)
FROM requests
WHERE user_id = $1
`
	var count int
	err := s.db.QueryRow(countRequestsQuery, userID).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Store) GetFirstRequestTime(userID int) (time.Time, error) {
	const getFirstRequestTimeQuery = `
SELECT created_at
FROM requests
WHERE user_id = $1
ORDER BY created_at ASC
LIMIT 1
`
	var firstRequest time.Time
	err := s.db.QueryRow(getFirstRequestTimeQuery, userID).Scan(&firstRequest)
	if err != nil {
		return time.Time{}, err
	}
	return firstRequest, nil
}
